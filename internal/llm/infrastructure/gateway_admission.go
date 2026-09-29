package infrastructure

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"sync"
	"time"

	d "InkLore/internal/llm/domain"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type GatewayRedisOptions struct {
	Client    redis.UniversalClient
	Recovery  d.RecoveryProbePort
	KeyHealth d.KeyHealthPort
}
type RedisAdmission struct {
	client   redis.UniversalClient
	recovery d.RecoveryProbePort
	health   d.KeyHealthPort
	mu       sync.Mutex
	dirty    map[string]bool
	known    map[string]bool
}

func NewRedisAdmission(o GatewayRedisOptions) (*RedisAdmission, error) {
	if o.Client == nil || o.Recovery == nil || o.KeyHealth == nil {
		return nil, errors.New("gateway admission dependencies required")
	}
	return &RedisAdmission{client: o.Client, recovery: o.Recovery, health: o.KeyHealth, dirty: map[string]bool{}, known: map[string]bool{}}, nil
}

var _ d.AdmissionPort = (*RedisAdmission)(nil)

func gatewayID(s string) bool {
	v, e := uuid.Parse(s)
	return e == nil && v.Version() == 4 && v.Variant() == uuid.RFC4122 && v.String() == s
}
func gatewayPrefix(tenant string) string { return "inklore:r1:llm:{" + tenant + "}:" }
func gatewayVersion(s string) string     { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func gatewayFailure(cause error, retry int64) error {
	if retry < 1 {
		retry = 1
	}
	if retry > 300 {
		retry = 300
	}
	return &d.AdmissionFailure{Cause: cause, RetryAfterSeconds: retry}
}

// A local connectivity observation only closes admission. It never grants quota
// or provides an in-memory fallback. The shared Redis guard and PG proof reopen it.
func (a *RedisAdmission) markFailed(tenant string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for t := range a.known {
		a.dirty[t] = true
	}
	a.dirty[tenant] = true
}
func (a *RedisAdmission) begin(ctx context.Context, tenant string) error {
	a.mu.Lock()
	needs := !a.known[tenant] || a.dirty[tenant]
	a.known[tenant] = true
	a.mu.Unlock()
	if !needs {
		return nil
	}
	_, e := a.client.Eval(ctx, gatewayRecoveryLua, []string{gatewayPrefix(tenant) + "guard"}, "begin", "0", "0").Result()
	if e != nil {
		a.markFailed(tenant)
		return gatewayFailure(d.ErrCapacityUnavailable, 30)
	}
	a.mu.Lock()
	delete(a.dirty, tenant)
	a.mu.Unlock()
	return nil
}

// ReconcileRecovery requires a fresh PG reservation query on every attempted
// reopening. No driver or domain table is accessed by this Redis adapter.
func (a *RedisAdmission) ReconcileRecovery(ctx context.Context, tenant string) error {
	if !gatewayID(tenant) {
		return d.ErrInvalid
	}
	snapshot, e := a.recovery.AdmissionRecovery(ctx, tenant)
	if e != nil {
		return gatewayFailure(d.ErrCapacityUnavailable, 30)
	}
	inflight := "0"
	if snapshot.Inflight {
		inflight = "1"
	}
	deadline := snapshot.LatestDeadline.UnixMilli()
	if snapshot.LatestDeadline.IsZero() {
		deadline = 0
	}
	values, e := a.eval(ctx, tenant, gatewayRecoveryLua, []string{gatewayPrefix(tenant) + "guard"}, "check", inflight, deadline)
	if e != nil {
		return e
	}
	if values[0] != "ok" {
		return gatewayFailure(d.ErrCapacityUnavailable, gatewayRetry(values))
	}
	a.mu.Lock()
	a.known[tenant] = true
	delete(a.dirty, tenant)
	a.mu.Unlock()
	return nil
}

type gatewayKey struct {
	ID          string `json:"id"`
	Version     string `json:"version"`
	Weight      int    `json:"weight"`
	RPM         int    `json:"rpm"`
	Concurrency int    `json:"concurrency"`
	State       int    `json:"state"`
	RPMKey      int    `json:"rpm_key"`
	LeaseKey    int    `json:"lease_key"`
}
type gatewayChannel struct {
	ID          string       `json:"id"`
	Priority    int          `json:"priority"`
	Weight      int          `json:"weight"`
	RPM         int          `json:"rpm"`
	Concurrency int          `json:"concurrency"`
	State       int          `json:"state"`
	RPMKey      int          `json:"rpm_key"`
	LeaseKey    int          `json:"lease_key"`
	Keys        []gatewayKey `json:"keys"`
}

func gatewayLimits(weight, rpm, concurrency int) bool {
	return weight >= 1 && weight <= 1000 && rpm >= 1 && rpm <= 100000 && concurrency >= 1 && concurrency <= 1000
}
func gatewayCandidates(in d.AdmissionRequest, token string) ([]string, []gatewayChannel, error) {
	if !gatewayID(in.TenantID) || !gatewayID(in.InvocationID) || !gatewayID(in.ModelID) || in.Attempt < 1 || in.Attempt > 4 || len(in.Candidates) == 0 || len(in.Candidates) > 1000 {
		return nil, nil, d.ErrInvalid
	}
	prefix := gatewayPrefix(in.TenantID)
	keys := []string{prefix + "guard", prefix + "attempt:" + in.InvocationID + ":" + strconv.Itoa(in.Attempt), prefix + "lease:" + token}
	groups := map[string]*gatewayChannel{}
	seen := map[string]bool{}
	index := func(names ...string) (int, int, int) {
		n := len(keys) + 1
		keys = append(keys, names...)
		return n, n + 1, n + 2
	}
	candidates := append([]d.Selection(nil), in.Candidates...)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].ChannelPriority != candidates[j].ChannelPriority {
			return candidates[i].ChannelPriority < candidates[j].ChannelPriority
		}
		if candidates[i].ChannelID != candidates[j].ChannelID {
			return candidates[i].ChannelID < candidates[j].ChannelID
		}
		return candidates[i].KeyID < candidates[j].KeyID
	})
	ordered := []*gatewayChannel{}
	for _, c := range candidates {
		version := in.KeyVersions[c.KeyID]
		if !gatewayID(c.ChannelID) || !gatewayID(c.KeyID) || c.ChannelPriority < 0 || c.ChannelPriority > 100 || !gatewayLimits(c.ChannelWeight, c.ChannelRPM, c.ChannelConcurrency) || !gatewayLimits(c.KeyWeight, c.KeyRPM, c.KeyConcurrency) || version == "" || len(version) > 256 || seen[c.KeyID] {
			return nil, nil, d.ErrInvalid
		}
		seen[c.KeyID] = true
		g := groups[c.ChannelID]
		if g == nil {
			g = &gatewayChannel{ID: c.ChannelID, Priority: c.ChannelPriority, Weight: c.ChannelWeight, RPM: c.ChannelRPM, Concurrency: c.ChannelConcurrency, Keys: []gatewayKey{}}
			g.State, g.RPMKey, g.LeaseKey = index(prefix+"channel:"+c.ChannelID+":state", prefix+"channel:"+c.ChannelID+":rpm", prefix+"channel:"+c.ChannelID+":leases")
			groups[c.ChannelID] = g
			ordered = append(ordered, g)
		} else if g.Priority != c.ChannelPriority || g.Weight != c.ChannelWeight || g.RPM != c.ChannelRPM || g.Concurrency != c.ChannelConcurrency {
			return nil, nil, d.ErrInvalid
		}
		k := gatewayKey{ID: c.KeyID, Version: version, Weight: c.KeyWeight, RPM: c.KeyRPM, Concurrency: c.KeyConcurrency}
		k.State, k.RPMKey, k.LeaseKey = index(prefix+"key:"+c.KeyID+":"+gatewayVersion(version)+":state", prefix+"key:"+c.KeyID+":rpm", prefix+"key:"+c.KeyID+":leases")
		g.Keys = append(g.Keys, k)
	}
	out := make([]gatewayChannel, 0, len(ordered))
	for _, g := range ordered {
		out = append(out, *g)
	}
	return keys, out, nil
}
func (a *RedisAdmission) Acquire(ctx context.Context, in d.AdmissionRequest) (d.AdmissionLease, error) {
	token := uuid.NewString()
	keys, groups, e := gatewayCandidates(in, token)
	if e != nil {
		return d.AdmissionLease{}, e
	}
	if e = a.begin(ctx, in.TenantID); e != nil {
		return d.AdmissionLease{}, e
	}
	raw, e := json.Marshal(groups)
	if e != nil {
		return d.AdmissionLease{}, d.ErrInvalid
	}
	values, e := a.eval(ctx, in.TenantID, gatewayAcquireLua, keys, string(raw), token, in.InvocationID+"/"+strconv.Itoa(in.Attempt))
	if e != nil {
		return d.AdmissionLease{}, e
	}
	if values[0] == "capacity" {
		return d.AdmissionLease{}, gatewayFailure(d.ErrCapacity, gatewayRetry(values))
	}
	if values[0] != "ok" {
		return d.AdmissionLease{}, gatewayFailure(d.ErrCapacityUnavailable, gatewayRetry(values))
	}
	if len(values) != 5 {
		return d.AdmissionLease{}, gatewayFailure(d.ErrCapacityUnavailable, 30)
	}
	millis, e := strconv.ParseInt(values[4], 10, 64)
	if e != nil {
		return d.AdmissionLease{}, gatewayFailure(d.ErrCapacityUnavailable, 30)
	}
	return d.AdmissionLease{TenantID: in.TenantID, InvocationID: in.InvocationID, ModelID: in.ModelID, ChannelID: values[1], KeyID: values[2], KeyVersion: values[3], Token: token, Attempt: in.Attempt, ExpiresAt: time.UnixMilli(millis).UTC()}, nil
}
func gatewayLeaseKeys(l d.AdmissionLease) ([]string, error) {
	if !gatewayID(l.TenantID) || !gatewayID(l.InvocationID) || !gatewayID(l.ChannelID) || !gatewayID(l.KeyID) || !gatewayID(l.Token) || l.KeyVersion == "" {
		return nil, d.ErrInvalid
	}
	p := gatewayPrefix(l.TenantID)
	return []string{p + "guard", p + "lease:" + l.Token, p + "channel:" + l.ChannelID + ":leases", p + "key:" + l.KeyID + ":leases", p + "channel:" + l.ChannelID + ":state", p + "key:" + l.KeyID + ":" + gatewayVersion(l.KeyVersion) + ":state"}, nil
}
func (a *RedisAdmission) lease(ctx context.Context, l d.AdmissionLease, action string, outcome d.ProviderOutcome) error {
	keys, e := gatewayLeaseKeys(l)
	if e != nil {
		return e
	}
	values, e := a.eval(ctx, l.TenantID, gatewayLeaseLua, keys, l.Token, action, outcome.Kind, outcome.RetryAfterSeconds, l.ChannelID, l.KeyID, l.KeyVersion)
	if e != nil {
		return e
	}
	if values[0] != "ok" {
		if action == "release" {
			return nil
		}
		return gatewayFailure(d.ErrCapacityUnavailable, 30)
	}
	return nil
}
func (a *RedisAdmission) Renew(ctx context.Context, l d.AdmissionLease) error {
	if !gatewayID(l.TenantID) {
		return d.ErrInvalid
	}
	if e := a.begin(ctx, l.TenantID); e != nil {
		return e
	}
	return a.lease(ctx, l, "renew", d.ProviderOutcome{})
}
func (a *RedisAdmission) Release(ctx context.Context, l d.AdmissionLease) error {
	return a.lease(ctx, l, "release", d.ProviderOutcome{})
}
func (a *RedisAdmission) Report(ctx context.Context, l d.AdmissionLease, outcome d.ProviderOutcome) error {
	switch outcome.Kind {
	case d.OutcomeSuccess, d.OutcomeServiceFailure, d.OutcomeNeutral:
	case d.OutcomeRateLimited:
		if outcome.RetryAfterSeconds < 1 {
			outcome.RetryAfterSeconds = 1
		}
		if outcome.RetryAfterSeconds > 300 {
			outcome.RetryAfterSeconds = 300
		}
	case d.OutcomeCredentialInvalid:
		if _, e := gatewayLeaseKeys(l); e != nil {
			return e
		}

	default:
		return d.ErrInvalid
	}
	if e := a.lease(ctx, l, "report", outcome); e != nil {
		return e
	}
	if outcome.Kind == d.OutcomeCredentialInvalid {
		return a.health.InvalidateCredential(ctx, l.TenantID, l.KeyID, l.KeyVersion)
	}
	return nil
}
func (a *RedisAdmission) eval(ctx context.Context, tenant, script string, keys []string, args ...any) ([]string, error) {
	raw, e := a.client.Eval(ctx, script, keys, args...).Result()
	if e != nil {
		a.markFailed(tenant)
		return nil, gatewayFailure(d.ErrCapacityUnavailable, 30)
	}
	items, ok := raw.([]any)
	if !ok || len(items) == 0 {
		a.markFailed(tenant)
		return nil, gatewayFailure(d.ErrCapacityUnavailable, 30)
	}
	values := make([]string, len(items))
	for i, v := range items {
		s, ok := v.(string)
		if !ok {
			a.markFailed(tenant)
			return nil, gatewayFailure(d.ErrCapacityUnavailable, 30)
		}
		values[i] = s
	}
	return values, nil
}
func gatewayRetry(values []string) int64 {
	if len(values) < 2 {
		return 30
	}
	n, e := strconv.ParseInt(values[1], 10, 64)
	if e != nil {
		return 30
	}
	return n
}

const gatewayAcquireLua = `
local clock=redis.call('TIME')
local now=tonumber(clock[1])*1000+math.floor(tonumber(clock[2])/1000)
local guard=KEYS[1]
if redis.call('HGET',guard,'ready')~='1' then return {'unavailable','30'} end
if redis.call('EXISTS',KEYS[2])==1 then return {'duplicate','30'} end
local groups=cjson.decode(ARGV[1])
local token=ARGV[2]
local rpmMember=ARGV[3]
local retry=300000
local function eligible(state,rpm,leases,rpmMax,conMax)
 redis.call('ZREMRANGEBYSCORE',rpm,'-inf',now-60000)
 redis.call('ZREMRANGEBYSCORE',leases,'-inf',now)
 if redis.call('HGET',state,'invalid')=='1' then return false end
 local blocked=tonumber(redis.call('HGET',state,'until') or '0')
 if blocked>now then retry=math.min(retry,blocked-now);return false end
 if redis.call('HGET',state,'mode')=='open' then
  local probeUntil=tonumber(redis.call('HGET',state,'probe_until') or '0')
  if probeUntil>now then retry=math.min(retry,probeUntil-now);return false end
 end
 if redis.call('ZCARD',rpm)>=rpmMax then
  local oldest=redis.call('ZRANGE',rpm,0,0,'WITHSCORES');retry=math.min(retry,tonumber(oldest[2])+60000-now);return false
 end
 if redis.call('ZCARD',leases)>=conMax then
  local oldest=redis.call('ZRANGE',leases,0,0,'WITHSCORES');retry=math.min(retry,tonumber(oldest[2])-now);return false
 end
 return true
end
local chosen=nil
local channelTotal=0
local priority=nil
for _,g in ipairs(groups) do
 if priority==nil or g.priority==priority then
  if eligible(KEYS[g.state],KEYS[g.rpm_key],KEYS[g.lease_key],g.rpm,g.concurrency) then
   g.available={}
   for _,k in ipairs(g.keys) do
    if eligible(KEYS[k.state],KEYS[k.rpm_key],KEYS[k.lease_key],k.rpm,k.concurrency) then table.insert(g.available,k) end
   end
   if #g.available>0 then
    priority=g.priority
    local current=tonumber(redis.call('HGET',KEYS[g.state],'current') or '0')+g.weight
    redis.call('HSET',KEYS[g.state],'current',current)
    channelTotal=channelTotal+g.weight
    if chosen==nil or current>chosen.current then g.current=current;chosen=g end
   end
  end
 end
end
if chosen==nil then return {'capacity',tostring(math.max(1,math.ceil(retry/1000)))} end
redis.call('HINCRBY',KEYS[chosen.state],'current',-channelTotal)
local selected=nil
local keyTotal=0
for _,k in ipairs(chosen.available) do
 local current=tonumber(redis.call('HGET',KEYS[k.state],'current') or '0')+k.weight
 redis.call('HSET',KEYS[k.state],'current',current)
 keyTotal=keyTotal+k.weight
 if selected==nil or current>selected.current then k.current=current;selected=k end
end
redis.call('HINCRBY',KEYS[selected.state],'current',-keyTotal)
local untilTime=now+30000
for _,node in ipairs({chosen,selected}) do
 redis.call('ZADD',KEYS[node.rpm_key],now,rpmMember)
 redis.call('PEXPIRE',KEYS[node.rpm_key],60001)
 redis.call('ZADD',KEYS[node.lease_key],untilTime,token)
 redis.call('PEXPIRE',KEYS[node.lease_key],30001)
 if redis.call('HGET',KEYS[node.state],'mode')=='open' then redis.call('HSET',KEYS[node.state],'probe',token,'probe_until',untilTime) end
end
redis.call('HSET',KEYS[3],'token',token,'channel',chosen.id,'key',selected.id,'version',selected.version,'until',untilTime)
redis.call('PEXPIRE',KEYS[3],30001)
redis.call('SET',KEYS[2],token,'EX',86400)
return {'ok',chosen.id,selected.id,selected.version,tostring(untilTime)}
`

const gatewayLeaseLua = `
local t=redis.call('TIME');local now=tonumber(t[1])*1000+math.floor(tonumber(t[2])/1000)
local token=ARGV[1];local action=ARGV[2]
if redis.call('HGET',KEYS[2],'token')~=token or redis.call('HGET',KEYS[2],'channel')~=ARGV[5] or redis.call('HGET',KEYS[2],'key')~=ARGV[6] or redis.call('HGET',KEYS[2],'version')~=ARGV[7] then return {'lost'} end
local expires=tonumber(redis.call('HGET',KEYS[2],'until') or '0')
if action=='renew' then
 if redis.call('HGET',KEYS[1],'ready')~='1' or expires<=now then return {'lost'} end
 if redis.call('ZSCORE',KEYS[3],token)==false or redis.call('ZSCORE',KEYS[4],token)==false then return {'lost'} end
 redis.call('ZADD',KEYS[3],now+30000,token);redis.call('ZADD',KEYS[4],now+30000,token)
 redis.call('PEXPIRE',KEYS[3],30001);redis.call('PEXPIRE',KEYS[4],30001)
 redis.call('HSET',KEYS[2],'until',now+30000);redis.call('PEXPIRE',KEYS[2],30001)
 for _,state in ipairs({KEYS[5],KEYS[6]}) do if redis.call('HGET',state,'probe')==token then redis.call('HSET',state,'probe_until',now+30000) end end
 return {'ok'}
end
if action=='release' then
 redis.call('ZREM',KEYS[3],token);redis.call('ZREM',KEYS[4],token);redis.call('DEL',KEYS[2])
 for _,state in ipairs({KEYS[5],KEYS[6]}) do if redis.call('HGET',state,'probe')==token then redis.call('HDEL',state,'probe','probe_until') end end
 return {'ok'}
end
if expires<=now then return {'lost'} end
if redis.call('HGET',KEYS[2],'reported')=='1' then return {'ok'} end
redis.call('HSET',KEYS[2],'reported',1)
local outcome=ARGV[3]
local retry=tonumber(ARGV[4])
for index,state in ipairs({KEYS[5],KEYS[6]}) do
 if outcome=='success' then
  if redis.call('HGET',state,'mode')~='open' or redis.call('HGET',state,'probe')==token then
   redis.call('HSET',state,'failures',0,'mode','closed','until',0,'cooldown',30000);redis.call('HDEL',state,'probe','probe_until')
  end
 elseif outcome=='service_failure' then
  local n=redis.call('HINCRBY',state,'failures',1)
  local probe=redis.call('HGET',state,'probe')==token
  if n>=5 or probe then
   local cooldown=30000
   if probe then cooldown=math.min(300000,tonumber(redis.call('HGET',state,'cooldown') or '30000')*2) end
   redis.call('HSET',state,'mode','open','until',now+cooldown,'cooldown',cooldown);redis.call('HDEL',state,'probe','probe_until')
  end
 elseif outcome=='rate_limited' then
  local deadline=math.max(tonumber(redis.call('HGET',state,'until') or '0'),now+retry*1000)
  redis.call('HSET',state,'until',deadline)
 elseif outcome=='credential_invalid' and index==2 then redis.call('HSET',state,'invalid',1)
 end
end
return {'ok'}
`

const gatewayRecoveryLua = `
local t=redis.call('TIME');local now=tonumber(t[1])*1000+math.floor(tonumber(t[2])/1000)
local mode=ARGV[1]
local exists=redis.call('EXISTS',KEYS[1])
local untilTime=tonumber(redis.call('HGET',KEYS[1],'until') or '0')
if mode=='begin' then
 untilTime=math.max(untilTime,now+30000)
 redis.call('HSET',KEYS[1],'ready',0,'until',untilTime)
elseif mode=='check' and exists==0 then
 untilTime=0
 redis.call('HSET',KEYS[1],'ready',1,'until',0)
end
if mode=='check' then
 if ARGV[2]=='1' then
  untilTime=math.max(untilTime,tonumber(ARGV[3]),now+30000)
  redis.call('HSET',KEYS[1],'ready',0,'until',untilTime)
 elseif now>=untilTime then redis.call('HSET',KEYS[1],'ready',1) end
end
if redis.call('HGET',KEYS[1],'ready')=='1' then return {'ok'} end
return {'unavailable',tostring(math.max(1,math.ceil((untilTime-now)/1000)))}
`
