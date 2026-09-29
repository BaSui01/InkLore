package domain

// Inspect evaluates every proposal without changing state. At most one safe conflict
// per proposal is returned; callers must reject the entire batch before any insert.
func Inspect(s State, proposals []FactProposal) []Conflict {
	conflicts := []Conflict{}
	seen := map[string]bool{}
	for i, p := range proposals {
		h := s.Head(p.SubjectID, p.Predicate, p.FactKey)
		var current *string
		if h != nil {
			x := h.EventID
			current = &x
		}
		reason := ""
		key := p.SubjectID + "/" + p.Predicate + "/" + p.FactKey
		if seen[key] {
			reason = "different_value"
		}
		seen[key] = true
		e := s.Entity(p.SubjectID)
		if e == nil || e.DeletedAt != nil {
			reason = "missing_entity"
		}
		if reason == "" && p.Predicate != "custom" && p.FactKey != p.Predicate {
			reason = "invalid_resolution"
		}
		if reason == "" {
			switch p.Action {
			case "assert":
				if p.TargetEventID != nil || p.Resolution != "none" || p.Value == nil {
					reason = "invalid_resolution"
				} else if h != nil && h.Status == "active" {
					reason = "different_value"
				}
			case "supersede", "retract":
				if p.TargetEventID == nil || h == nil || h.Status != "active" || *p.TargetEventID != h.EventID {
					reason = "head_changed"
				} else if !One(p.Resolution, "correction", "retcon") || p.Reason == nil || !Text(*p.Reason, 1, 1000) || (p.Action == "retract") != (p.Value == nil) {
					reason = "invalid_resolution"
				}
			default:
				reason = "invalid_resolution"
			}
		}
		if reason == "" && e != nil && p.Value != nil {
			reason = inspectValue(s, *e, h, p)
		}
		if reason != "" {
			conflicts = append(conflicts, Conflict{FactKey: p.FactKey, SubjectID: p.SubjectID, CurrentEventID: current, ProposalIndex: int64(i), Reason: reason})
		}
	}
	return conflicts
}

// ValidateProjection checks the final replay state independently of cached
// character fields. Retcon bypasses transitions only, never entity/type checks.
func ValidateProjection(s State, heads []FactHead) error {
	reason := "projection_rebuild"
	for _, h := range heads {
		if h.Status != "active" {
			continue
		}
		e := s.Entity(h.SubjectID)
		if e == nil || e.DeletedAt != nil || h.Value == nil {
			return Corrupt()
		}
		p := FactProposal{SubjectID: h.SubjectID, Predicate: h.Predicate, FactKey: h.FactKey, Value: h.Value, Resolution: "retcon", Reason: &reason}
		if h.Predicate != "custom" && h.FactKey != h.Predicate {
			return Corrupt()
		}
		if inspectValue(s, *e, nil, p) != "" {
			return Corrupt()
		}
	}
	return nil
}
func inspectValue(s State, e Entity, h *FactHead, p FactProposal) string {
	v := p.Value
	if v.Validate() != nil {
		return "invalid_resolution"
	}
	if v.Kind == "entity" {
		other := s.Entity(*v.EntityID)
		if other == nil || other.DeletedAt != nil {
			return "missing_entity"
		}
	}
	retcon := p.Resolution == "retcon" && p.Reason != nil && Text(*p.Reason, 1, 1000)
	switch p.Predicate {
	case "life_status":
		if e.Kind != "character" || v.Kind != "text" || !One(*v.TextValue, "alive", "missing", "dead", "revived") {
			return "invalid_transition"
		}
		from, to := e.LifeStatus, *v.TextValue
		normal := from == to || (from == "alive" && One(to, "missing", "dead")) || (from == "missing" && One(to, "alive", "dead")) || (from == "revived" && One(to, "missing", "dead"))
		if !normal && !retcon {
			return "invalid_transition"
		}
	case "realm_rank":
		if e.Kind != "character" || v.Kind != "number" {
			return "invalid_transition"
		}
		if *v.NumberValue < e.RealmRank && !retcon {
			return "rank_regression"
		}
	case "item_status":
		if e.Kind != "lore" || e.Category != "item" || v.Kind != "text" || !One(*v.TextValue, "intact", "damaged", "destroyed") {
			return "invalid_transition"
		}
		from := "intact"
		if h != nil && h.Status == "active" && h.Value != nil && h.Value.TextValue != nil {
			from = *h.Value.TextValue
		}
		to := *v.TextValue
		if !(from == to || (from == "intact" && One(to, "damaged", "destroyed")) || (from == "damaged" && to == "destroyed")) && !retcon {
			return "invalid_transition"
		}
	case "location":
		if v.Kind != "entity" {
			return "invalid_transition"
		}
		other := s.Entity(*v.EntityID)
		if other == nil || other.Kind != "lore" || other.Category != "place" {
			return "invalid_transition"
		}
	case "relationship":
		if v.Kind != "entity" || *v.EntityID == e.ID {
			return "invalid_transition"
		}
	case "custom":
	default:
		return "invalid_resolution"
	}
	return ""
}
