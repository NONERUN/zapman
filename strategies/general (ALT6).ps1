Import-Module -Force -DisableNameChecking (Join-Path (Split-Path -Parent $PSScriptRoot) 'src\Zapman\Zapman.psd1')
Invoke-ZapretStrategyPrep
$layout = Get-ZapretLayout
$bin = $layout.Bin
$lists = $layout.Lists
$user = $layout.User
$gf = Get-ZapretGameFilter
$argList = @"
--wf-tcp=80,443,2053,2083,2087,2096,8443,$($gf.Tcp) --wf-udp=443,19294-19344,50000-50100,$($gf.Udp)
--filter-udp=443 --hostlist="$lists\list-general.txt" --hostlist="$user\list-general-user.txt" --hostlist-exclude="$lists\list-exclude.txt" --hostlist-exclude="$user\list-exclude-user.txt" --ipset-exclude="$lists\ipset-exclude.txt" --ipset-exclude="$user\ipset-exclude-user.txt" --dpi-desync=fake --dpi-desync-repeats=6 --dpi-desync-fake-quic="$bin\quic_initial_www_google_com.bin" --new
--filter-udp=19294-19344,50000-50100 --filter-l7=discord,stun --dpi-desync=fake --dpi-desync-fake-discord="$bin\ACTIVE_DISCORD_UDP.bin" --dpi-desync-fake-stun="$bin\ACTIVE_DISCORD_UDP.bin" --dpi-desync-repeats=6 --new
--filter-tcp=2053,2083,2087,2096,8443 --hostlist-domains=discord.media --dpi-desync=multisplit --dpi-desync-split-seqovl=681 --dpi-desync-split-pos=1 --dpi-desync-split-seqovl-pattern="$bin\tls_clienthello_www_google_com.bin" --new
--filter-tcp=443 --hostlist="$lists\list-google.txt" --ip-id=zero --dpi-desync=multisplit --dpi-desync-split-seqovl=681 --dpi-desync-split-pos=1 --dpi-desync-split-seqovl-pattern="$bin\tls_clienthello_www_google_com.bin" --new
--filter-tcp=80,443 --hostlist="$lists\list-general.txt" --hostlist="$user\list-general-user.txt" --hostlist-exclude="$lists\list-exclude.txt" --hostlist-exclude="$user\list-exclude-user.txt" --ipset-exclude="$lists\ipset-exclude.txt" --ipset-exclude="$user\ipset-exclude-user.txt" --dpi-desync=multisplit --dpi-desync-split-seqovl=681 --dpi-desync-split-pos=1 --dpi-desync-split-seqovl-pattern="$bin\tls_clienthello_www_google_com.bin" --new
--filter-udp=443 --ipset="$user\ipset-all.txt" --hostlist-exclude="$lists\list-exclude.txt" --hostlist-exclude="$user\list-exclude-user.txt" --ipset-exclude="$lists\ipset-exclude.txt" --ipset-exclude="$user\ipset-exclude-user.txt" --dpi-desync=fake --dpi-desync-repeats=6 --dpi-desync-fake-quic="$bin\quic_initial_www_google_com.bin" --new
--filter-tcp=80,443,8443 --ipset="$user\ipset-all.txt" --hostlist-exclude="$lists\list-exclude.txt" --hostlist-exclude="$user\list-exclude-user.txt" --ipset-exclude="$lists\ipset-exclude.txt" --ipset-exclude="$user\ipset-exclude-user.txt" --dpi-desync=multisplit --dpi-desync-split-seqovl=681 --dpi-desync-split-pos=1 --dpi-desync-split-seqovl-pattern="$bin\tls_clienthello_www_google_com.bin" --new
--filter-tcp=$($gf.Tcp) --ipset="$user\ipset-all.txt" --ipset-exclude="$lists\ipset-exclude.txt" --ipset-exclude="$user\ipset-exclude-user.txt" --dpi-desync=multisplit --dpi-desync-any-protocol=1 --dpi-desync-cutoff=n3 --dpi-desync-split-seqovl=681 --dpi-desync-split-pos=1 --dpi-desync-split-seqovl-pattern="$bin\tls_clienthello_www_google_com.bin" --new
--filter-udp=$($gf.Udp) --ipset="$user\ipset-all.txt" --ipset-exclude="$lists\ipset-exclude.txt" --ipset-exclude="$user\ipset-exclude-user.txt" --dpi-desync=fake --dpi-desync-repeats=12 --dpi-desync-any-protocol=1 --dpi-desync-fake-unknown-udp="$bin\ACTIVE_GAME_UDP.bin" --dpi-desync-cutoff=n2
"@
Start-ZapretWinws -ArgumentList $argList
