# Diagnostic HTTP smoke for the running acceptance-only browser fixture.
# This supplies browser-like headers manually and is NOT browser evidence.
$ErrorActionPreference = 'Stop'
$session = [Microsoft.PowerShell.Commands.WebRequestSession]::new()
function Check($condition, $label) { if (-not $condition) { throw "Acceptance failed: $label" } }
function Request($method, $origin, $path, $headers, $body = $null, $jar = $session) {
    # PowerShell retains custom headers on WebRequestSession; browsers do not.
    $jar.Headers.Clear()
    $args = @{ Method=$method; Uri=($origin+$path); Headers=$headers; WebSession=$jar; SkipHttpErrorCheck=$true; TimeoutSec=15 }
    if ($null -ne $body) { $args.Body=($body | ConvertTo-Json -Compress); $args.ContentType='application/json' }
    Invoke-WebRequest @args
}
$creatorOrigin='http://localhost:3000'
$adminOrigin='http://localhost:3001'
$creatorHeaders=@{Origin=$creatorOrigin;'Sec-Fetch-Site'='same-origin'}
$adminHeaders=@{Origin=$adminOrigin;'Sec-Fetch-Site'='same-origin'}
$creatorLogin=Request POST $creatorOrigin '/__acceptance/login?audience=creator' $creatorHeaders
Check ($creatorLogin.StatusCode -eq 200) 'creator fixture login'
$creator=($creatorLogin.Content|ConvertFrom-Json).data
$adminLogin=Request POST $adminOrigin '/__acceptance/login?audience=admin' $adminHeaders
Check ($adminLogin.StatusCode -eq 200) 'admin fixture login'
$admin=($adminLogin.Content|ConvertFrom-Json).data
$cookies=$session.Cookies.GetCookies([uri]($creatorOrigin+'/v1/auth'))
Check ($cookies.Count -eq 2) 'two audience cookies in one jar'
foreach($cookie in $cookies) { Check ($cookie.HttpOnly -and $cookie.Path -eq '/v1/auth') 'HttpOnly refresh cookie path' }
Write-Output 'PASS fixture_real_login_and_dual_cookie_jar'
$recoveryHeaders=@{'Sec-Fetch-Site'='same-origin'}
$recovered=Request GET $creatorOrigin '/v1/auth/session-csrf?audience=creator' $recoveryHeaders
Check ($recovered.StatusCode -eq 200) 'no-Origin creator recovery through proxy'
Check ((($recovered.Content|ConvertFrom-Json).data.csrf_token) -eq $creator.csrf_token) 'stable recovered CSRF'
$cross=Request GET $creatorOrigin '/v1/auth/session-csrf?audience=admin' $recoveryHeaders
Check ($cross.StatusCode -eq 403) 'cross audience cookie cannot restore'
$untrusted=Request GET $creatorOrigin '/v1/auth/session-csrf?audience=creator' @{}
Check ($untrusted.StatusCode -eq 403) 'missing Origin and Fetch Metadata refused'
Write-Output 'PASS proxy_recovery_and_cross_audience_rejection'
$creatorHeaders['X-CSRF-Token']=$creator.csrf_token
$refreshed=Request POST $creatorOrigin '/v1/auth/refresh' $creatorHeaders @{audience='creator'}
Check ($refreshed.StatusCode -eq 200) 'real cookie-jar rotation'
$logout=Request POST $creatorOrigin '/v1/auth/logout' $creatorHeaders @{audience='creator'}
Check ($logout.StatusCode -eq 200) 'creator logout'
$again=Request POST $creatorOrigin '/v1/auth/logout' $creatorHeaders @{audience='creator'}
Check ($again.StatusCode -eq 200) 'idempotent logout after cookie deletion'
$adminRecovery=Request GET $adminOrigin '/v1/auth/session-csrf?audience=admin' $recoveryHeaders
Check ($adminRecovery.StatusCode -eq 200) 'admin cookie survives creator logout'
Write-Output 'PASS rotation_logout_and_other_audience_survival'
$adminCookie=$session.Cookies.GetCookies([uri]($adminOrigin+'/v1/auth'))['inklore_admin_refresh']
$oldValue=$adminCookie.Value
$adminHeaders['X-CSRF-Token']=$admin.csrf_token
$renewed=Request POST $adminOrigin '/v1/auth/refresh' $adminHeaders @{audience='admin'}
Check ($renewed.StatusCode -eq 200) 'admin refresh'
$newAccess=($renewed.Content|ConvertFrom-Json).data.access_token
$oldJar=[Microsoft.PowerShell.Commands.WebRequestSession]::new()
$oldJar.Cookies.Add([uri]$adminOrigin,[System.Net.Cookie]::new('inklore_admin_refresh',$oldValue,'/v1/auth'))
$replay=Request POST $adminOrigin '/v1/auth/refresh' $adminHeaders @{audience='admin'} $oldJar
Check ($replay.StatusCode -eq 401 -and ($replay.Content|ConvertFrom-Json).code -eq 'auth.refresh_reused') 'old refresh rejected'
$me=Request GET $adminOrigin '/v1/auth/me' @{Authorization=('Bearer '+$newAccess)}
Check ($me.StatusCode -eq 401) 'replay revocation committed'
Write-Output 'PASS cookie_jar_replay_revokes_rotated_access'
Write-Output 'HTTP fixture smoke: 4 groups passed; no real browser was used.'
