# Strategy JSON to winws / winws2 argv. Load by path. Do not start winws from this module.

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

$script:ZapretSpecAllowedDesync = @{
    fake          = $true
    multisplit    = $true
    multidisorder = $true
    hostfakesplit = $true
    fakedsplit    = $true
    syndata       = $true
}

$script:ZapretSpecFakeSlotFlag = @{
    quic       = '--dpi-desync-fake-quic'
    tls        = '--dpi-desync-fake-tls'
    http       = '--dpi-desync-fake-http'
    discord    = '--dpi-desync-fake-discord'
    stun       = '--dpi-desync-fake-stun'
    unknown    = '--dpi-desync-fake-unknown'
    unknownUdp = '--dpi-desync-fake-unknown-udp'
}

$script:ZapretSpecFakeSlotOrder = @(
    'quic'
    'tls'
    'http'
    'discord'
    'stun'
    'unknown'
    'unknownUdp'
)

# z1 '!' is the built-in TLS ClientHello. z2 has no built-in hello. Use the stock google hello.
$script:ZapretSpecDefaultTlsFake = 'bin:tls_clienthello_www_google_com.bin'

function Get-ZapretSpecProperty {
    param(
        $Object,
        [string]$Name
    )
    if ($null -eq $Object) {
        return $null
    }
    $prop = $Object.PSObject.Properties[$Name]
    if ($null -eq $prop) {
        return $null
    }
    return $prop.Value
}

function Test-ZapretSpecHasProperty {
    param(
        $Object,
        [string]$Name
    )
    if ($null -eq $Object) {
        return $false
    }
    return [bool]$Object.PSObject.Properties[$Name]
}

function Get-ZapretSpecStringList {
    param($Value)
    if ($null -eq $Value) {
        return @()
    }
    $out = New-Object System.Collections.ArrayList
    foreach ($item in @($Value)) {
        if ($null -eq $item) {
            continue
        }
        [void]$out.Add([string]$item)
    }
    return @($out)
}

function Get-ZapretSpecVersion {
    $mod = $MyInvocation.MyCommand.Module
    if ($null -eq $mod) {
        return 'v0.0.0'
    }
    $num = [string]$mod.Version
    if ([string]::IsNullOrWhiteSpace($num)) {
        return 'v0.0.0'
    }
    return ('v{0}' -f $num)
}

function Get-ZapretSpecModuleVersionNumber {
    $tag = Get-ZapretSpecVersion
    if ($tag.Length -ge 1 -and ($tag[0] -eq 'v' -or $tag[0] -eq 'V')) {
        return $tag.Substring(1)
    }
    return $tag
}

function Get-ZapretStrategySpec {
    param([string]$Path)
    if ([string]::IsNullOrWhiteSpace($Path)) {
        throw 'Strategy JSON path is empty.'
    }
    if (-not (Test-Path -LiteralPath $Path)) {
        throw ("Strategy JSON not found: {0}" -f $Path)
    }
    $raw = [System.IO.File]::ReadAllText($Path)
    try {
        $spec = $raw | ConvertFrom-Json
    } catch {
        throw ("Strategy JSON is not valid: {0}" -f $Path)
    }
    if ($null -eq $spec) {
        throw ("Strategy JSON is empty: {0}" -f $Path)
    }
    $want = Get-ZapretSpecModuleVersionNumber
    $got = [string](Get-ZapretSpecProperty -Object $spec -Name 'specVersion')
    if ([string]::IsNullOrWhiteSpace($got)) {
        throw ("Strategy JSON has no specVersion: {0}" -f $Path)
    }
    if ($got -ne $want) {
        throw ("Strategy JSON specVersion '{0}' does not match ZapretSpec {1}: {2}" -f $got, $want, $Path)
    }
    return $spec
}

function ConvertFrom-ZapretSpecPathRef {
    param(
        [string]$Ref,
        [string]$Bin,
        [string]$Lists,
        [string]$User
    )
    $t = [string]$Ref
    if ($t.Length -ge 6 -and $t.Substring(0, 6) -eq 'lists:') {
        return (Join-Path $Lists $t.Substring(6))
    }
    if ($t.Length -ge 5 -and $t.Substring(0, 5) -eq 'user:') {
        return (Join-Path $User $t.Substring(5))
    }
    if ($t.Length -ge 4 -and $t.Substring(0, 4) -eq 'bin:') {
        return (Join-Path $Bin $t.Substring(4))
    }
    return $null
}

function Test-ZapretSpecFileRef {
    param([string]$Ref)
    if ([string]::IsNullOrWhiteSpace($Ref)) {
        return $false
    }
    if ($Ref.StartsWith('lists:') -or $Ref.StartsWith('user:') -or $Ref.StartsWith('bin:')) {
        return $true
    }
    return $false
}

function Get-ZapretSpecBlobStem {
    param([string]$Ref)
    if ($Ref -match '^0x([0-9A-Fa-f]+)$') {
        return ('hex_{0}' -f $matches[1])
    }
    if (-not (Test-ZapretSpecFileRef -Ref $Ref)) {
        $safe = ($Ref -replace '[^A-Za-z0-9]+', '_')
        $safe = $safe.Trim('_')
        if ([string]::IsNullOrWhiteSpace($safe)) {
            return 'blob'
        }
        $first = $safe.Substring(0, 1)
        if ($first -match '[0-9]') {
            return ('b_{0}' -f $safe)
        }
        return $safe
    }
    $name = $Ref.Substring($Ref.IndexOf(':') + 1)
    $file = Split-Path -Leaf $name
    $dot = $file.LastIndexOf('.')
    if ($dot -gt 0) {
        return $file.Substring(0, $dot)
    }
    return $file
}

function ConvertTo-ZapretSpecQuotedPath {
    param([string]$Path)
    return ('"{0}"' -f $Path)
}

function ConvertTo-ZapretSpecFilterValue {
    param(
        [string]$Value,
        [string]$GameFilterTcp,
        [string]$GameFilterUdp
    )
    $t = [string]$Value
    $t = $t.Replace('$gf.Tcp', [string]$GameFilterTcp)
    $t = $t.Replace('$gf.Udp', [string]$GameFilterUdp)
    return $t
}

function ConvertTo-ZapretSpecCsv {
    param(
        [string[]]$Items,
        [string]$GameFilterTcp,
        [string]$GameFilterUdp
    )
    $parts = New-Object System.Collections.ArrayList
    foreach ($item in @($Items)) {
        if ([string]::IsNullOrWhiteSpace($item)) {
            continue
        }
        [void]$parts.Add((ConvertTo-ZapretSpecFilterValue -Value $item -GameFilterTcp $GameFilterTcp -GameFilterUdp $GameFilterUdp))
    }
    return (@($parts) -join ',')
}

function Get-ZapretSpecDesyncList {
    param($SpecProfile)
    $names = @(Get-ZapretSpecStringList -Value (Get-ZapretSpecProperty -Object $SpecProfile -Name 'desync'))
    if (@($names).Count -lt 1) {
        throw 'Strategy profile has no desync attacks.'
    }
    foreach ($name in $names) {
        if (-not $script:ZapretSpecAllowedDesync.ContainsKey($name)) {
            throw ("Unknown desync attack '{0}'." -f $name)
        }
    }
    return @($names)
}

function Get-ZapretSpecFakeMap {
    param($SpecProfile)
    $fake = Get-ZapretSpecProperty -Object $SpecProfile -Name 'fake'
    $map = @{}
    if ($null -eq $fake) {
        return $map
    }
    foreach ($slot in $script:ZapretSpecFakeSlotOrder) {
        if (-not (Test-ZapretSpecHasProperty -Object $fake -Name $slot)) {
            continue
        }
        $map[$slot] = @(Get-ZapretSpecStringList -Value (Get-ZapretSpecProperty -Object $fake -Name $slot))
    }
    return $map
}

function Add-ZapretSpecBlobToRegistry {
    param(
        [string]$Ref,
        $Order,
        $Names,
        $Values,
        [string]$Bin,
        [string]$Lists,
        [string]$User
    )
    if ([string]::IsNullOrWhiteSpace($Ref)) {
        return
    }
    if ($Ref -eq '!') {
        $def = $script:ZapretSpecDefaultTlsFake
        Add-ZapretSpecBlobToRegistry -Ref $def -Order $Order -Names $Names -Values $Values -Bin $Bin -Lists $Lists -User $User
        if ($Names.ContainsKey($def)) {
            $Names['!'] = $Names[$def]
        }
        return
    }
    if ($Names.ContainsKey($Ref)) {
        return
    }
    $stem = Get-ZapretSpecBlobStem -Ref $Ref
    $base = $stem
    $n = 2
    $used = @{}
    foreach ($k in @($Names.Keys)) {
        $used[$Names[$k]] = $true
    }
    while ($used.ContainsKey($stem)) {
        $stem = ('{0}_{1}' -f $base, $n)
        $n++
    }
    $path = ConvertFrom-ZapretSpecPathRef -Ref $Ref -Bin $Bin -Lists $Lists -User $User
    if ($null -ne $path) {
        $Values[$stem] = '@' + (ConvertTo-ZapretSpecQuotedPath -Path $path)
    } else {
        $Values[$stem] = $Ref
    }
    $Names[$Ref] = $stem
    [void]$Order.Add($stem)
}

function Add-ZapretSpecWinwsFakeFlags {
    param(
        [System.Collections.ArrayList]$Parts,
        $FakeMap,
        [string]$Bin,
        [string]$Lists,
        [string]$User
    )
    foreach ($slot in $script:ZapretSpecFakeSlotOrder) {
        if (-not $FakeMap.ContainsKey($slot)) {
            continue
        }
        $flag = $script:ZapretSpecFakeSlotFlag[$slot]
        foreach ($ref in @($FakeMap[$slot])) {
            if ([string]::IsNullOrWhiteSpace($ref)) {
                continue
            }
            $path = ConvertFrom-ZapretSpecPathRef -Ref $ref -Bin $Bin -Lists $Lists -User $User
            if ($null -ne $path) {
                [void]$Parts.Add(('{0}={1}' -f $flag, (ConvertTo-ZapretSpecQuotedPath -Path $path)))
            } else {
                [void]$Parts.Add(('{0}={1}' -f $flag, $ref))
            }
        }
    }
}

function Add-ZapretSpecCommonFilters {
    param(
        [System.Collections.ArrayList]$Parts,
        $SpecProfile,
        [string]$Bin,
        [string]$Lists,
        [string]$User,
        [string]$GameFilterTcp,
        [string]$GameFilterUdp
    )
    $l3 = Get-ZapretSpecProperty -Object $SpecProfile -Name 'l3'
    if (-not [string]::IsNullOrWhiteSpace([string]$l3)) {
        [void]$Parts.Add(('--filter-l3={0}' -f [string]$l3))
    }
    $udp = Get-ZapretSpecProperty -Object $SpecProfile -Name 'udp'
    if (-not [string]::IsNullOrWhiteSpace([string]$udp)) {
        [void]$Parts.Add(('--filter-udp={0}' -f (ConvertTo-ZapretSpecFilterValue -Value ([string]$udp) -GameFilterTcp $GameFilterTcp -GameFilterUdp $GameFilterUdp)))
    }
    $tcp = Get-ZapretSpecProperty -Object $SpecProfile -Name 'tcp'
    if (-not [string]::IsNullOrWhiteSpace([string]$tcp)) {
        [void]$Parts.Add(('--filter-tcp={0}' -f (ConvertTo-ZapretSpecFilterValue -Value ([string]$tcp) -GameFilterTcp $GameFilterTcp -GameFilterUdp $GameFilterUdp)))
    }
    $l7 = Get-ZapretSpecProperty -Object $SpecProfile -Name 'l7'
    if (-not [string]::IsNullOrWhiteSpace([string]$l7)) {
        [void]$Parts.Add(('--filter-l7={0}' -f [string]$l7))
    }

    $listKeys = @(
        @{ Name = 'hostlist'; Flag = '--hostlist' }
        @{ Name = 'hostlistExclude'; Flag = '--hostlist-exclude' }
        @{ Name = 'ipset'; Flag = '--ipset' }
        @{ Name = 'ipsetExclude'; Flag = '--ipset-exclude' }
    )
    foreach ($pair in $listKeys) {
        foreach ($ref in @(Get-ZapretSpecStringList -Value (Get-ZapretSpecProperty -Object $SpecProfile -Name $pair.Name))) {
            $path = ConvertFrom-ZapretSpecPathRef -Ref $ref -Bin $Bin -Lists $Lists -User $User
            if ($null -eq $path) {
                throw ("Path prefix is not lists:/user:/bin: : {0}" -f $ref)
            }
            [void]$Parts.Add(('{0}={1}' -f $pair.Flag, (ConvertTo-ZapretSpecQuotedPath -Path $path)))
        }
    }

    $domains = @(Get-ZapretSpecStringList -Value (Get-ZapretSpecProperty -Object $SpecProfile -Name 'hostlistDomains'))
    if (@($domains).Count -gt 0) {
        [void]$Parts.Add(('--hostlist-domains={0}' -f ($domains -join ',')))
    }
    $exDomains = @(Get-ZapretSpecStringList -Value (Get-ZapretSpecProperty -Object $SpecProfile -Name 'hostlistExcludeDomains'))
    if (@($exDomains).Count -gt 0) {
        [void]$Parts.Add(('--hostlist-exclude-domains={0}' -f ($exDomains -join ',')))
    }
}

function Get-ZapretSpecFoolingList {
    param($SpecProfile)
    return @(Get-ZapretSpecStringList -Value (Get-ZapretSpecProperty -Object $SpecProfile -Name 'fooling'))
}

function ConvertTo-ZapretSpecLuaFoolingArgs {
    param($SpecProfile)
    $luaArgs = New-Object System.Collections.ArrayList
    $fooling = @(Get-ZapretSpecFoolingList -SpecProfile $SpecProfile)
    foreach ($item in $fooling) {
        if ($item -eq 'ts') {
            [void]$luaArgs.Add('tcp_ts=-600000')
        } elseif ($item -eq 'md5sig') {
            [void]$luaArgs.Add('tcp_md5')
        } elseif ($item -eq 'badseq') {
            $inc = Get-ZapretSpecProperty -Object $SpecProfile -Name 'badseqIncrement'
            if ($null -ne $inc) {
                [void]$luaArgs.Add(('tcp_seq={0}' -f $inc))
                [void]$luaArgs.Add(('tcp_ack={0}' -f $inc))
            }
        } else {
            throw ("Unknown fooling '{0}'." -f $item)
        }
    }
    return @($luaArgs)
}

function Test-ZapretSpecAttackTakesFooling {
    param([string]$Name)
    return ($Name -ne 'multisplit')
}

function Get-ZapretSpecLuaModArgs {
    param(
        [string]$Attack,
        $SpecProfile,
        $BlobNames
    )
    $luaArgs = New-Object System.Collections.ArrayList
    if ($Attack -eq 'hostfakesplit') {
        $mod = [string](Get-ZapretSpecProperty -Object $SpecProfile -Name 'hostfakesplitMod')
        if (-not [string]::IsNullOrWhiteSpace($mod)) {
            foreach ($piece in @($mod -split ',')) {
                $p = $piece.Trim()
                if ($p) {
                    [void]$luaArgs.Add($p)
                }
            }
        }
    }
    if ($Attack -eq 'fakedsplit') {
        $pat = [string](Get-ZapretSpecProperty -Object $SpecProfile -Name 'fakedsplitPattern')
        if (-not [string]::IsNullOrWhiteSpace($pat)) {
            if (Test-ZapretSpecFileRef -Ref $pat) {
                [void]$luaArgs.Add(('pattern={0}' -f $BlobNames[$pat]))
            } else {
                [void]$luaArgs.Add(('pattern={0}' -f $pat))
            }
        }
    }
    $splitPos = Get-ZapretSpecProperty -Object $SpecProfile -Name 'splitPos'
    if (($Attack -eq 'multisplit' -or $Attack -eq 'multidisorder' -or $Attack -eq 'fakedsplit') -and -not [string]::IsNullOrWhiteSpace([string]$splitPos)) {
        [void]$luaArgs.Add(('pos={0}' -f [string]$splitPos))
    }
    $seqovl = Get-ZapretSpecProperty -Object $SpecProfile -Name 'seqovl'
    if (($Attack -eq 'multisplit' -or $Attack -eq 'multidisorder' -or $Attack -eq 'fakedsplit') -and $null -ne $seqovl) {
        [void]$luaArgs.Add(('seqovl={0}' -f $seqovl))
    }
    $seqPat = [string](Get-ZapretSpecProperty -Object $SpecProfile -Name 'seqovlPattern')
    if (($Attack -eq 'multisplit' -or $Attack -eq 'multidisorder' -or $Attack -eq 'fakedsplit') -and -not [string]::IsNullOrWhiteSpace($seqPat)) {
        if (-not $BlobNames.ContainsKey($seqPat)) {
            throw ("seqovlPattern blob is missing: {0}" -f $seqPat)
        }
        [void]$luaArgs.Add(('seqovl_pattern={0}' -f $BlobNames[$seqPat]))
    }
    return @($luaArgs)
}

function Register-ZapretSpecBlobs {
    param(
        $Spec,
        [string]$Bin,
        [string]$Lists,
        [string]$User
    )
    $order = New-Object System.Collections.ArrayList
    $names = @{}
    $values = @{}

    foreach ($specProfile in @(Get-ZapretSpecProperty -Object $Spec -Name 'profiles')) {
        $fakeMap = Get-ZapretSpecFakeMap -SpecProfile $specProfile
        foreach ($slot in $script:ZapretSpecFakeSlotOrder) {
            if (-not $fakeMap.ContainsKey($slot)) {
                continue
            }
            foreach ($ref in @($fakeMap[$slot])) {
                Add-ZapretSpecBlobToRegistry -Ref $ref -Order $order -Names $names -Values $values -Bin $Bin -Lists $Lists -User $User
            }
        }
        $seqPat = [string](Get-ZapretSpecProperty -Object $specProfile -Name 'seqovlPattern')
        if (-not [string]::IsNullOrWhiteSpace($seqPat)) {
            Add-ZapretSpecBlobToRegistry -Ref $seqPat -Order $order -Names $names -Values $values -Bin $Bin -Lists $Lists -User $User
        }
        $fakePat = [string](Get-ZapretSpecProperty -Object $specProfile -Name 'fakedsplitPattern')
        if ((Test-ZapretSpecFileRef -Ref $fakePat)) {
            Add-ZapretSpecBlobToRegistry -Ref $fakePat -Order $order -Names $names -Values $values -Bin $Bin -Lists $Lists -User $User
        }
    }

    return New-Object PSObject -Property @{
        Order  = @($order)
        Names  = $names
        Values = $values
    }
}

function Get-ZapretSpecPayloadGroups {
    param($SpecProfile)
    $payload = [string](Get-ZapretSpecProperty -Object $SpecProfile -Name 'payload')
    if (-not [string]::IsNullOrWhiteSpace($payload)) {
        return @($payload)
    }
    $any = Get-ZapretSpecProperty -Object $SpecProfile -Name 'anyProtocol'
    if ([bool]$any) {
        return @('all')
    }
    $l7 = [string](Get-ZapretSpecProperty -Object $SpecProfile -Name 'l7')
    if ($l7 -match 'tls' -and $l7 -match 'http') {
        return @('tls_client_hello', 'http_req')
    }
    if ($l7 -match 'quic') {
        return @('quic_initial')
    }
    if ($l7 -match 'discord' -or $l7 -match 'stun') {
        return @('discord_ip_discovery,stun')
    }
    if ($l7 -match 'tls') {
        return @('tls_client_hello')
    }
    if ($l7 -match 'http') {
        return @('http_req')
    }
    return @('')
}

function Get-ZapretSpecFakeRefsForGroup {
    param(
        $FakeMap,
        [string]$Group
    )
    $slots = @($script:ZapretSpecFakeSlotOrder)
    if ($Group -eq 'tls_client_hello') {
        $slots = @('tls')
    } elseif ($Group -eq 'http_req') {
        $slots = @('http')
    } elseif ($Group -eq 'quic_initial') {
        $slots = @('quic')
    } elseif ($Group -eq 'discord_ip_discovery,stun') {
        $slots = @('discord', 'stun')
    }
    $seen = @{}
    $refs = New-Object System.Collections.ArrayList
    foreach ($slot in $slots) {
        if (-not $FakeMap.ContainsKey($slot)) {
            continue
        }
        foreach ($ref in @($FakeMap[$slot])) {
            if ([string]::IsNullOrWhiteSpace($ref)) {
                continue
            }
            if ($seen.ContainsKey($ref)) {
                continue
            }
            $seen[$ref] = $true
            [void]$refs.Add($ref)
        }
    }
    return @($refs)
}

function Test-ZapretSpecNeedOutRange {
    param($SpecProfile)
    $cutoff = [string](Get-ZapretSpecProperty -Object $SpecProfile -Name 'cutoff')
    if ($cutoff -match '^n\d+$') {
        return $true
    }
    $tcp = [string](Get-ZapretSpecProperty -Object $SpecProfile -Name 'tcp')
    if (-not [string]::IsNullOrWhiteSpace($tcp)) {
        return $true
    }
    $l7 = [string](Get-ZapretSpecProperty -Object $SpecProfile -Name 'l7')
    if ($l7 -match 'quic') {
        return $true
    }
    $payload = [string](Get-ZapretSpecProperty -Object $SpecProfile -Name 'payload')
    if ($payload -match 'quic') {
        return $true
    }
    $fakeMap = Get-ZapretSpecFakeMap -SpecProfile $SpecProfile
    if ($fakeMap.ContainsKey('quic')) {
        return $true
    }
    return $false
}

function ConvertTo-ZapretSpecWinwsArgList {
    param(
        $Spec,
        [string]$Bin,
        [string]$Lists,
        [string]$User,
        [string]$GameFilterTcp,
        [string]$GameFilterUdp
    )
    $lines = New-Object System.Collections.ArrayList
    $wf = Get-ZapretSpecProperty -Object $Spec -Name 'wf'
    if ($null -eq $wf) {
        throw 'Strategy JSON has no wf.'
    }
    $wfTcp = ConvertTo-ZapretSpecCsv -Items @(Get-ZapretSpecStringList -Value (Get-ZapretSpecProperty -Object $wf -Name 'tcp')) -GameFilterTcp $GameFilterTcp -GameFilterUdp $GameFilterUdp
    $wfUdp = ConvertTo-ZapretSpecCsv -Items @(Get-ZapretSpecStringList -Value (Get-ZapretSpecProperty -Object $wf -Name 'udp')) -GameFilterTcp $GameFilterTcp -GameFilterUdp $GameFilterUdp
    [void]$lines.Add(('--wf-tcp={0} --wf-udp={1}' -f $wfTcp, $wfUdp))

    $specList = @(Get-ZapretSpecProperty -Object $Spec -Name 'profiles')
    $last = @($specList).Count - 1
    $i = 0
    foreach ($specProfile in $specList) {
        $parts = New-Object System.Collections.ArrayList
        Add-ZapretSpecCommonFilters -Parts $parts -SpecProfile $specProfile -Bin $Bin -Lists $Lists -User $User -GameFilterTcp $GameFilterTcp -GameFilterUdp $GameFilterUdp

        $ipId = Get-ZapretSpecProperty -Object $specProfile -Name 'ipId'
        if (-not [string]::IsNullOrWhiteSpace([string]$ipId)) {
            [void]$parts.Add(('--ip-id={0}' -f [string]$ipId))
        }

        $desync = @(Get-ZapretSpecDesyncList -SpecProfile $specProfile)
        [void]$parts.Add(('--dpi-desync={0}' -f ($desync -join ',')))

        $repeats = Get-ZapretSpecProperty -Object $specProfile -Name 'repeats'
        if ($null -ne $repeats) {
            [void]$parts.Add(('--dpi-desync-repeats={0}' -f $repeats))
        }
        $any = Get-ZapretSpecProperty -Object $specProfile -Name 'anyProtocol'
        if ([bool]$any) {
            [void]$parts.Add('--dpi-desync-any-protocol=1')
        }
        $cutoff = Get-ZapretSpecProperty -Object $specProfile -Name 'cutoff'
        if (-not [string]::IsNullOrWhiteSpace([string]$cutoff)) {
            [void]$parts.Add(('--dpi-desync-cutoff={0}' -f [string]$cutoff))
        }
        $seqovl = Get-ZapretSpecProperty -Object $specProfile -Name 'seqovl'
        if ($null -ne $seqovl) {
            [void]$parts.Add(('--dpi-desync-split-seqovl={0}' -f $seqovl))
        }
        $splitPos = Get-ZapretSpecProperty -Object $specProfile -Name 'splitPos'
        if (-not [string]::IsNullOrWhiteSpace([string]$splitPos)) {
            [void]$parts.Add(('--dpi-desync-split-pos={0}' -f [string]$splitPos))
        }
        $fooling = @(Get-ZapretSpecFoolingList -SpecProfile $specProfile)
        if (@($fooling).Count -gt 0) {
            [void]$parts.Add(('--dpi-desync-fooling={0}' -f ($fooling -join ',')))
        }
        $badseq = Get-ZapretSpecProperty -Object $specProfile -Name 'badseqIncrement'
        if ($null -ne $badseq) {
            [void]$parts.Add(('--dpi-desync-badseq-increment={0}' -f $badseq))
        }
        $seqPat = [string](Get-ZapretSpecProperty -Object $specProfile -Name 'seqovlPattern')
        if (-not [string]::IsNullOrWhiteSpace($seqPat)) {
            $path = ConvertFrom-ZapretSpecPathRef -Ref $seqPat -Bin $Bin -Lists $Lists -User $User
            if ($null -eq $path) {
                throw ("seqovlPattern must be a bin:/lists:/user: path: {0}" -f $seqPat)
            }
            [void]$parts.Add(('--dpi-desync-split-seqovl-pattern={0}' -f (ConvertTo-ZapretSpecQuotedPath -Path $path)))
        }
        $hfs = Get-ZapretSpecProperty -Object $specProfile -Name 'hostfakesplitMod'
        if (-not [string]::IsNullOrWhiteSpace([string]$hfs)) {
            [void]$parts.Add(('--dpi-desync-hostfakesplit-mod={0}' -f [string]$hfs))
        }
        $fds = Get-ZapretSpecProperty -Object $specProfile -Name 'fakedsplitPattern'
        if (-not [string]::IsNullOrWhiteSpace([string]$fds)) {
            [void]$parts.Add(('--dpi-desync-fakedsplit-pattern={0}' -f [string]$fds))
        }

        Add-ZapretSpecWinwsFakeFlags -Parts $parts -FakeMap (Get-ZapretSpecFakeMap -SpecProfile $specProfile) -Bin $Bin -Lists $Lists -User $User

        $tlsMod = Get-ZapretSpecProperty -Object $specProfile -Name 'tlsMod'
        if (-not [string]::IsNullOrWhiteSpace([string]$tlsMod)) {
            [void]$parts.Add(('--dpi-desync-fake-tls-mod={0}' -f [string]$tlsMod))
        }

        $line = (@($parts) -join ' ')
        if ($i -lt $last) {
            $line = $line + ' --new'
        }
        [void]$lines.Add($line)
        $i++
    }
    return (@($lines) -join [Environment]::NewLine)
}

function ConvertTo-ZapretSpecWinws2ArgList {
    param(
        $Spec,
        [string]$Bin,
        [string]$Lists,
        [string]$User,
        [string]$GameFilterTcp,
        [string]$GameFilterUdp
    )
    $lines = New-Object System.Collections.ArrayList
    $wf = Get-ZapretSpecProperty -Object $Spec -Name 'wf'
    if ($null -eq $wf) {
        throw 'Strategy JSON has no wf.'
    }
    $wfTcp = ConvertTo-ZapretSpecCsv -Items @(Get-ZapretSpecStringList -Value (Get-ZapretSpecProperty -Object $wf -Name 'tcp')) -GameFilterTcp $GameFilterTcp -GameFilterUdp $GameFilterUdp
    $wfUdp = ConvertTo-ZapretSpecCsv -Items @(Get-ZapretSpecStringList -Value (Get-ZapretSpecProperty -Object $wf -Name 'udp')) -GameFilterTcp $GameFilterTcp -GameFilterUdp $GameFilterUdp
    [void]$lines.Add(('--wf-tcp-out={0} --wf-udp-out={1}' -f $wfTcp, $wfUdp))

    $lib = Join-Path $Bin 'lua\zapret-lib.lua'
    $anti = Join-Path $Bin 'lua\zapret-antidpi.lua'
    [void]$lines.Add(('--lua-init=@{0} --lua-init=@{1}' -f (ConvertTo-ZapretSpecQuotedPath -Path $lib), (ConvertTo-ZapretSpecQuotedPath -Path $anti)))

    $blobs = Register-ZapretSpecBlobs -Spec $Spec -Bin $Bin -Lists $Lists -User $User
    $needEmpty = $false
    foreach ($specProfile in @(Get-ZapretSpecProperty -Object $Spec -Name 'profiles')) {
        $desync = @(Get-ZapretSpecDesyncList -SpecProfile $specProfile)
        if ($desync -notcontains 'fake') {
            continue
        }
        $fakeMap = Get-ZapretSpecFakeMap -SpecProfile $specProfile
        $count = 0
        foreach ($slot in $script:ZapretSpecFakeSlotOrder) {
            if (-not $fakeMap.ContainsKey($slot)) {
                continue
            }
            foreach ($ref in @($fakeMap[$slot])) {
                if ($ref -eq '!') {
                    continue
                }
                $count++
            }
        }
        if ($count -lt 1) {
            $needEmpty = $true
        }
    }
    if ($needEmpty -and -not $blobs.Values.ContainsKey('empty')) {
        $blobs.Values['empty'] = '0x00'
        $arr = New-Object System.Collections.ArrayList
        foreach ($n in @($blobs.Order)) {
            [void]$arr.Add($n)
        }
        [void]$arr.Add('empty')
        $blobs.Order = @($arr)
    }

    # zapret2: --blob=name:@file or --blob=name:0xHEX
    $blobParts = New-Object System.Collections.ArrayList
    foreach ($name in @($blobs.Order)) {
        [void]$blobParts.Add(('--blob={0}:{1}' -f $name, $blobs.Values[$name]))
    }
    if (@($blobParts).Count -gt 0) {
        [void]$lines.Add((@($blobParts) -join ' '))
    }

    $specList = @(Get-ZapretSpecProperty -Object $Spec -Name 'profiles')
    $last = @($specList).Count - 1
    $i = 0
    foreach ($specProfile in $specList) {
        $parts = New-Object System.Collections.ArrayList
        Add-ZapretSpecCommonFilters -Parts $parts -SpecProfile $specProfile -Bin $Bin -Lists $Lists -User $User -GameFilterTcp $GameFilterTcp -GameFilterUdp $GameFilterUdp

        $cutoff = [string](Get-ZapretSpecProperty -Object $specProfile -Name 'cutoff')
        if ($cutoff -match '^n(\d+)$') {
            [void]$parts.Add(('--out-range=-d{0}' -f $matches[1]))
        } elseif (Test-ZapretSpecNeedOutRange -SpecProfile $specProfile) {
            [void]$parts.Add('--out-range=-d10')
        }

        $desync = @(Get-ZapretSpecDesyncList -SpecProfile $specProfile)
        $fakeMap = Get-ZapretSpecFakeMap -SpecProfile $specProfile
        $tlsLast = $null
        if ($fakeMap.ContainsKey('tls')) {
            $tlsRefs = @($fakeMap['tls'])
            if ($tlsRefs.Count -gt 0) {
                $tlsLast = [string]$tlsRefs[$tlsRefs.Count - 1]
            }
        }
        $repeats = Get-ZapretSpecProperty -Object $specProfile -Name 'repeats'
        $ipId = Get-ZapretSpecProperty -Object $specProfile -Name 'ipId'
        $tlsMod = [string](Get-ZapretSpecProperty -Object $specProfile -Name 'tlsMod')
        $foolArgs = @(ConvertTo-ZapretSpecLuaFoolingArgs -SpecProfile $specProfile)
        $groups = @(Get-ZapretSpecPayloadGroups -SpecProfile $specProfile)

        foreach ($group in $groups) {
            if (-not [string]::IsNullOrWhiteSpace($group)) {
                [void]$parts.Add(('--payload={0}' -f $group))
            }

            foreach ($attack in $desync) {
                if ($attack -eq 'fake') {
                    $refs = @(Get-ZapretSpecFakeRefsForGroup -FakeMap $fakeMap -Group $group)
                    $emitted = $false
                    foreach ($ref in $refs) {
                            $lua = New-Object System.Collections.ArrayList
                            [void]$lua.Add('fake')
                            if ($blobs.Names.ContainsKey($ref)) {
                                [void]$lua.Add(('blob={0}' -f $blobs.Names[$ref]))
                            } else {
                                [void]$lua.Add('blob=empty')
                            }
                            if ($null -ne $repeats) {
                                [void]$lua.Add(('repeats={0}' -f $repeats))
                            }
                            if (-not [string]::IsNullOrWhiteSpace($tlsMod) -and $ref -eq $tlsLast) {
                                [void]$lua.Add(('tls_mod={0}' -f $tlsMod))
                            }
                            if (Test-ZapretSpecAttackTakesFooling -Name 'fake') {
                                foreach ($f in $foolArgs) {
                                    [void]$lua.Add($f)
                                }
                            }
                            if (-not [string]::IsNullOrWhiteSpace([string]$ipId)) {
                                [void]$lua.Add(('ip_id={0}' -f [string]$ipId))
                            }
                            [void]$parts.Add(('--lua-desync={0}' -f (@($lua) -join ':')))
                            $emitted = $true
                    }
                    if (-not $emitted) {
                        $lua = New-Object System.Collections.ArrayList
                        [void]$lua.Add('fake')
                        [void]$lua.Add('blob=empty')
                        if ($null -ne $repeats) {
                            [void]$lua.Add(('repeats={0}' -f $repeats))
                        }
                        if (-not [string]::IsNullOrWhiteSpace($tlsMod)) {
                            [void]$lua.Add(('tls_mod={0}' -f $tlsMod))
                        }
                        foreach ($f in $foolArgs) {
                            [void]$lua.Add($f)
                        }
                        if (-not [string]::IsNullOrWhiteSpace([string]$ipId)) {
                            [void]$lua.Add(('ip_id={0}' -f [string]$ipId))
                        }
                        [void]$parts.Add(('--lua-desync={0}' -f (@($lua) -join ':')))
                    }
                    continue
                }

                $lua = New-Object System.Collections.ArrayList
                [void]$lua.Add($attack)
                foreach ($extra in @(Get-ZapretSpecLuaModArgs -Attack $attack -SpecProfile $specProfile -BlobNames $blobs.Names)) {
                    [void]$lua.Add($extra)
                }
                if ($null -ne $repeats -and $attack -ne 'multisplit' -and $attack -ne 'multidisorder') {
                    [void]$lua.Add(('repeats={0}' -f $repeats))
                }
                if (Test-ZapretSpecAttackTakesFooling -Name $attack) {
                    foreach ($f in $foolArgs) {
                        [void]$lua.Add($f)
                    }
                }
                if (-not [string]::IsNullOrWhiteSpace([string]$ipId)) {
                    [void]$lua.Add(('ip_id={0}' -f [string]$ipId))
                }
                [void]$parts.Add(('--lua-desync={0}' -f (@($lua) -join ':')))
            }
        }

        $line = (@($parts) -join ' ')
        if ($i -lt $last) {
            $line = $line + ' --new'
        }
        [void]$lines.Add($line)
        $i++
    }
    return (@($lines) -join [Environment]::NewLine)
}

function ConvertTo-ZapretStrategyArgList {
    param(
        $Spec,
        [ValidateSet('winws', 'winws2')]
        [string]$Engine,
        [string]$Bin,
        [string]$Lists,
        [string]$User,
        [string]$GameFilterTcp,
        [string]$GameFilterUdp
    )
    if ($null -eq $Spec) {
        throw 'Strategy spec is empty.'
    }
    if ([string]::IsNullOrWhiteSpace($Engine)) {
        throw 'Engine is empty.'
    }
    $common = @{
        Spec           = $Spec
        Bin            = $Bin
        Lists          = $Lists
        User           = $User
        GameFilterTcp  = $GameFilterTcp
        GameFilterUdp  = $GameFilterUdp
    }
    if ($Engine -eq 'winws2') {
        return ConvertTo-ZapretSpecWinws2ArgList @common
    }
    return ConvertTo-ZapretSpecWinwsArgList @common
}

Export-ModuleMember -Function @(
    'Get-ZapretSpecVersion'
    'Get-ZapretStrategySpec'
    'ConvertTo-ZapretStrategyArgList'
)
