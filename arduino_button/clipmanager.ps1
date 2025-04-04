# Laad de System.Web assembly voor URL-codering (indien beschikbaar)
$useSystemWeb = $true
try {
    Add-Type -AssemblyName System.Web
    Write-Host "✅ System.Web geladen"
} catch {
    Write-Host "❌ Fout bij het laden van System.Web: $_"
    Write-Host "Gebruik fallback voor URL-codering."
    $useSystemWeb = $false
}

$baudRate = 9600

# Bepaal de server-URL en chat-configuratie
function Get-Config {
    $configPath = Join-Path -Path $PSScriptRoot -ChildPath "config.json"
    $config = $null

    # 1. Probeer config.json te lezen
    if (Test-Path $configPath) {
        try {
            $config = Get-Content -Path $configPath -Raw | ConvertFrom-Json
            Write-Host "✅ Configuratie geladen uit config.json"
        } catch {
            Write-Host "❌ Fout bij het lezen van config.json: $_"
        }
    } else {
        Write-Host "⚠️ config.json niet gevonden."
    }

    # 2. Valideer de configuratie
    if (-not $config) {
        Write-Host "❌ Fout: config.json kon niet worden geladen."
        Write-Host "   1. Zorg ervoor dat config.json bestaat in de arduino_button map."
        Write-Host "   2. Controleer of het bestand geldige JSON bevat."
        Write-Host "Druk op een toets om af te sluiten..."
        $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
        exit 1
    }

    # Valideer ServerUrl
    if (-not $config.ServerUrl -or $config.ServerUrl -eq "") {
        Write-Host "❌ Fout: ServerUrl is niet ingesteld in config.json."
        Write-Host "   1. Open config.json in de arduino_button map."
        Write-Host "   2. Stel de ServerUrl in, bijvoorbeeld: {`"ServerUrl`": `"http://jouw-server:5001`"}"
        Write-Host "   3. Sla het bestand op en herstart dit script."
        Write-Host "Druk op een toets om af te sluiten..."
        $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
        exit 1
    }

    # Valideer ChatApp
    if (-not $config.ChatApp -or $config.ChatApp -eq "") {
        Write-Host "❌ Fout: ChatApp is niet ingesteld in config.json."
        Write-Host "   1. Open config.json in de arduino_button map."
        Write-Host "   2. Stel de ChatApp in, bijvoorbeeld: {`"ChatApp`": `"mattermost`"}"
        Write-Host "   3. Sla het bestand op en herstart dit script."
        Write-Host "Druk op een toets om af te sluiten..."
        $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
        exit 1
    }

    # Valideer ChatAppConfig
    if (-not $config.ChatAppConfig) {
        Write-Host "❌ Fout: ChatAppConfig is niet ingesteld in config.json."
        Write-Host "   1. Open config.json in de arduino_button map."
        Write-Host "   2. Voeg een ChatAppConfig object toe met de juiste parameters voor jouw chatplatform."
        Write-Host "   3. Sla het bestand op en herstart dit script."
        Write-Host "Druk op een toets om af te sluiten..."
        $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
        exit 1
    }

    # Valideer platform-specifieke parameters
    switch ($config.ChatApp.ToLower()) {
        "mattermost" {
            if (-not $config.ChatAppConfig.mattermost_url -or $config.ChatAppConfig.mattermost_url -eq "" -or
                -not $config.ChatAppConfig.mattermost_channel -or $config.ChatAppConfig.mattermost_channel -eq "" -or
                -not $config.ChatAppConfig.mattermost_token -or $config.ChatAppConfig.mattermost_token -eq "") {
                Write-Host "❌ Fout: Mattermost parameters (mattermost_url, mattermost_channel, mattermost_token) zijn niet volledig ingesteld in config.json."
                Write-Host "   1. Open config.json in de arduino_button map."
                Write-Host "   2. Stel de Mattermost parameters in onder ChatAppConfig, bijvoorbeeld:"
                Write-Host '      "ChatAppConfig": {'
                Write-Host '        "mattermost_url": "https://mm.your-server.com",'
                Write-Host '        "mattermost_channel": "your-channel-id",'
                Write-Host '        "mattermost_token": "your-token"'
                Write-Host '      }'
                Write-Host "   3. Sla het bestand op en herstart dit script."
                Write-Host "Druk op een toets om af te sluiten..."
                $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
                exit 1
            }
        }
        "telegram" {
            if (-not $config.ChatAppConfig.telegram_bot_token -or $config.ChatAppConfig.telegram_bot_token -eq "" -or
                -not $config.ChatAppConfig.telegram_chat_id -or $config.ChatAppConfig.telegram_chat_id -eq "") {
                Write-Host "❌ Fout: Telegram parameters (telegram_bot_token, telegram_chat_id) zijn niet volledig ingesteld in config.json."
                Write-Host "   1. Open config.json in de arduino_button map."
                Write-Host "   2. Stel de Telegram parameters in onder ChatAppConfig, bijvoorbeeld:"
                Write-Host '      "ChatAppConfig": {'
                Write-Host '        "telegram_bot_token": "your-bot-token",'
                Write-Host '        "telegram_chat_id": "your-chat-id"'
                Write-Host '      }'
                Write-Host "   3. Sla het bestand op en herstart dit script."
                Write-Host "Druk op een toets om af te sluiten..."
                $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
                exit 1
            }
        }
        "discord" {
            if (-not $config.ChatAppConfig.discord_webhook_url -or $config.ChatAppConfig.discord_webhook_url -eq "") {
                Write-Host "❌ Fout: Discord parameter (discord_webhook_url) is niet ingesteld in config.json."
                Write-Host "   1. Open config.json in de arduino_button map."
                Write-Host "   2. Stel de Discord parameter in onder ChatAppConfig, bijvoorbeeld:"
                Write-Host '      "ChatAppConfig": {'
                Write-Host '        "discord_webhook_url": "your-webhook-url"'
                Write-Host '      }'
                Write-Host "   3. Sla het bestand op en herstart dit script."
                Write-Host "Druk op een toets om af te sluiten..."
                $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
                exit 1
            }
        }
        default {
            Write-Host "❌ Fout: Ongeldige ChatApp waarde in config.json: $($config.ChatApp)"
            Write-Host "   Geldige waarden zijn: 'mattermost', 'telegram', 'discord'."
            Write-Host "   1. Open config.json in de arduino_button map."
            Write-Host "   2. Stel een geldige ChatApp in, bijvoorbeeld: {`"ChatApp`": `"mattermost`"}"
            Write-Host "   3. Sla het bestand op en herstart dit script."
            Write-Host "Druk op een toets om af te sluiten..."
            $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
            exit 1
        }
    }

    # Valideer BacktrackSeconds en DurationSeconds
    if (-not $config.BacktrackSeconds -or $config.BacktrackSeconds -lt 0 -or $config.BacktrackSeconds -gt 300) {
        Write-Host "❌ Fout: BacktrackSeconds moet een getal zijn tussen 0 en 300 in config.json."
        Write-Host "   1. Open config.json in de arduino_button map."
        Write-Host "   2. Stel een geldige BacktrackSeconds in, bijvoorbeeld: {`"BacktrackSeconds`": 60}"
        Write-Host "   3. Sla het bestand op en herstart dit script."
        Write-Host "Druk op een toets om af te sluiten..."
        $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
        exit 1
    }

    if (-not $config.DurationSeconds -or $config.DurationSeconds -lt 1 -or $config.DurationSeconds -gt 300) {
        Write-Host "❌ Fout: DurationSeconds moet een getal zijn tussen 1 en 300 in config.json."
        Write-Host "   1. Open config.json in de arduino_button map."
        Write-Host "   2. Stel een geldige DurationSeconds in, bijvoorbeeld: {`"DurationSeconds`": 60}"
        Write-Host "   3. Sla het bestand op en herstart dit script."
        Write-Host "Druk op een toets om af te sluiten..."
        $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
        exit 1
    }

    # 3. Als er geen server-URL is gevonden, probeer een lokale server (bijv. http://localhost:5001)
    $serverUrl = $config.ServerUrl
    if (-not (Test-ServerUrl -url $serverUrl)) {
        $localServerUrl = "http://localhost:5001"
        Write-Host "⚠️ ServerUrl ($serverUrl) niet bereikbaar, probeer lokale server: $localServerUrl"
        if (Test-ServerUrl -url $localServerUrl) {
            $serverUrl = $localServerUrl
            Write-Host "✅ Lokale server gevonden: $serverUrl"
        } else {
            Write-Host "❌ Lokale server niet bereikbaar: $localServerUrl"
            Write-Host "❌ Fout: Geen geldige ServerUrl gevonden."
            Write-Host "   1. Controleer of de ClipManager-server draait."
            Write-Host "   2. Controleer de ServerUrl in config.json."
            Write-Host "   3. Sla het bestand op en herstart dit script."
            Write-Host "Druk op een toets om af te sluiten..."
            $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
            exit 1
        }
    }

    return $config
}

# Test of een server-URL bereikbaar is
function Test-ServerUrl {
    param($url)
    try {
        $response = Invoke-WebRequest -Uri "$url/api/clip" -Method Get -UseBasicParsing -TimeoutSec 2
        if ($response.StatusCode -eq 200) {
            return $true
        }
    } catch {
        return $false
    }
    return $false
}

# Haal de configuratie op
$config = Get-Config
$serverUrl = $config.ServerUrl
$chatApp = $config.ChatApp.ToLower()
$backtrackSeconds = $config.BacktrackSeconds
$durationSeconds = $config.DurationSeconds
$team1 = if ($config.Team1) { [System.Web.HttpUtility]::UrlEncode($config.Team1) } else { "" }
$team2 = if ($config.Team2) { [System.Web.HttpUtility]::UrlEncode($config.Team2) } else { "" }
$additionalText = if ($config.AdditionalText) { [System.Web.HttpUtility]::UrlEncode($config.AdditionalText) } else { "" }

# Bouw de API-endpoint URL dynamisch op basis van de chat-app
$apiEndpointBase = "$serverUrl/api/clip?chat_app=$chatApp&category={0}&backtrack_seconds=$backtrackSeconds&duration_seconds=$durationSeconds"
if ($team1) { $apiEndpointBase += "&team1=$team1" }
if ($team2) { $apiEndpointBase += "&team2=$team2" }
if ($additionalText) { $apiEndpointBase += "&additional_text=$additionalText" }

switch ($chatApp) {
    "mattermost" {
        $mattermostUrl = $config.ChatAppConfig.mattermost_url
        $mattermostChannel = $config.ChatAppConfig.mattermost_channel
        $mattermostToken = $config.ChatAppConfig.mattermost_token
        $apiEndpoint = "$apiEndpointBase&mattermost_url=$mattermostUrl&mattermost_channel=$mattermostChannel&mattermost_token=$mattermostToken"
    }
    "telegram" {
        $telegramBotToken = $config.ChatAppConfig.telegram_bot_token
        $telegramChatId = $config.ChatAppConfig.telegram_chat_id
        $apiEndpoint = "$apiEndpointBase&telegram_bot_token=$telegramBotToken&telegram_chat_id=$telegramChatId"
    }
    "discord" {
        $discordWebhookUrl = $config.ChatAppConfig.discord_webhook_url
        $apiEndpoint = "$apiEndpointBase&discord_webhook_url=$discordWebhookUrl"
    }
}

# Fallback functie voor URL-codering als System.Web niet beschikbaar is
function UrlEncode {
    param($text)
    $chars = $text.ToCharArray()
    $encoded = ""
    foreach ($char in $chars) {
        if ($char -match '[a-zA-Z0-9]') {
            $encoded += $char
        } else {
            $encoded += "%" + [System.Convert]::ToByte($char).ToString("X2")
        }
    }
    return $encoded
}

function Find-ArduinoPorts {
    $ports = [System.IO.Ports.SerialPort]::GetPortNames()
    $arduinoPorts = @{}

    Write-Host "Beschikbare poorten: $($ports -join ', ')"
    foreach ($portName in $ports) {
        try {
            Write-Host "Probeer poort $portName..."
            $port = New-Object System.IO.Ports.SerialPort $portName, $baudRate, 'None', 8, 'One'
            $port.ReadTimeout = 1000
            $port.WriteTimeout = 1000
            $port.Open()
            Start-Sleep -Milliseconds 500
            $port.WriteLine("IDENTIFY")
            Start-Sleep -Milliseconds 500
            $response = $port.ReadLine().Trim()
            Write-Host "Antwoord van ${portName}: $response"
            $port.Close()
            if ($response.StartsWith("CLIPMANAGER_")) {
                Write-Host "✅ Arduino gevonden op $portName met identifier $response"
                $arduinoPorts[$portName] = $response
            } else {
                Write-Host "❌ Geen geldige identifier op $portName"
            }
        } catch {
            Write-Host "❌ Fout bij het openen van ${portName}: $_"
            if ($port.IsOpen) { $port.Close() }
        }
    }

    if ($arduinoPorts.Count -eq 0) {
        Write-Host "❌ Geen Arduino's gevonden, blijf scannen..."
    } else {
        Write-Host "Gevonden Arduino's: $($arduinoPorts.Count)"
        foreach ($portName in $arduinoPorts.Keys) {
            Write-Host "Poort ${portName}: $($arduinoPorts[$portName])"
        }
    }
    return $arduinoPorts
}

function Monitor-Port {
    param($portName, $identifier)

    # Haal de categorie uit de identifier (alles achter "CLIPMANAGER_")
    try {
        if ($identifier -match "^CLIPMANAGER_(.+)$") {
            $categoryRaw = $matches[1] # Bijv. "HYPE" of "BLUNDER"
            # URL-encode de categorie
            if ($useSystemWeb) {
                $category = [System.Web.HttpUtility]::UrlEncode($categoryRaw)
            } else {
                $category = UrlEncode -text $categoryRaw
            }
            Write-Host "[$portName] Categorie: $category"
        } else {
            Write-Host "[$portName] ❌ Ongeldige identifier: $identifier"
            return
        }
    } catch {
        Write-Host "[$portName] ❌ Fout bij het bepalen van de categorie: $_"
        return
    }

    $port = New-Object System.IO.Ports.SerialPort $portName, $baudRate, 'None', 8, 'One'
    $port.ReadTimeout = 1000

    try {
        $port.Open()
        Write-Host "✅ Thread gestart voor $portName ($identifier)"
    } catch {
        Write-Host "❌ Kon ${portName} niet openen voor ${identifier}: $_"
        return
    }

    while ($true) {
        try {
            $line = $port.ReadLine().Trim()
            Write-Host "[$portName] Ontvangen: $line"

            if ($line -eq "BUTTON_PRESSED") {
                Write-Host "[$portName] 🟢 Request wordt verstuurd voor $identifier..."
                try {
                    # Gebruik de dynamische URL met de categorie
                    $requestUrl = [string]::Format($apiEndpoint, $category)
                    Invoke-WebRequest -Uri $requestUrl -UseBasicParsing

                    Write-Host "[$portName] ✅ Verzoek verstuurd om $(Get-Date -Format 'HH:mm:ss')"
                    Start-Sleep -Seconds 5
                } catch {
                    Write-Host "[$portName] ❌ Fout bij verzoek: $_"
                }
            }
        } catch {
            Write-Host "[$portName] ⚠️ Leesfout: $_"
            if (-not $port.IsOpen) {
                Write-Host "[$portName] 🔄 Poort is gesloten, stoppen met monitoren..."
                break
            } else {
                Start-Sleep -Milliseconds 500
            }
        }
    }
}

# Hoofdloop: blijf zoeken naar Arduino's
try {
    Write-Host "ClipManager is gestart en draait op de achtergrond. Sluit dit venster niet als je het ziet."
    Write-Host "Script gestart, zoeken naar Arduino's..."
    while ($true) {
        # Zoek alle Arduino's
        $arduinoPorts = Find-ArduinoPorts

        # Start een thread voor elke nieuwe Arduino die nog niet wordt gemonitord
        $runningJobs = Get-Job | Where-Object { $_.State -eq "Running" }
        foreach ($portName in $arduinoPorts.Keys) {
            $identifier = $arduinoPorts[$portName]
            $jobExists = $runningJobs | Where-Object { $_.Command -like "*$portName*" }
            if (-not $jobExists) {
                Write-Host "Start nieuwe thread voor $portName ($identifier)"
                Start-Job -ScriptBlock {
                    param($portName, $identifier, $useSystemWeb, $apiEndpoint)

                    # Fallback functie voor URL-codering
                    function UrlEncode {
                        param($text)
                        $chars = $text.ToCharArray()
                        $encoded = ""
                        foreach ($char in $chars) {
                            if ($char -match '[a-zA-Z0-9]') {
                                $encoded += $char
                            } else {
                                $encoded += "%" + [System.Convert]::ToByte($char).ToString("X2")
                            }
                        }
                        return $encoded
                    }

                    # Laad System.Web in de thread (indien beschikbaar)
                    if ($useSystemWeb) {
                        try {
                            Add-Type -AssemblyName System.Web
                        } catch {
                            Write-Host "[$portName] ❌ Fout bij het laden van System.Web in thread: $_"
                            $useSystemWeb = $false
                        }
                    }

                    # Definieer de Monitor-Port functie in de thread
                    function Monitor-Port {
                        param($portName, $identifier)

                        # Haal de categorie uit de identifier (alles achter "CLIPMANAGER_")
                        try {
                            if ($identifier -match "^CLIPMANAGER_(.+)$") {
                                $categoryRaw = $matches[1] # Bijv. "HYPE" of "BLUNDER"
                                # URL-encode de categorie
                                if ($useSystemWeb) {
                                    $category = [System.Web.HttpUtility]::UrlEncode($categoryRaw)
                                } else {
                                    $category = UrlEncode -text $categoryRaw
                                }
                                Write-Host "[$portName] Categorie: $category"
                            } else {
                                Write-Host "[$portName] ❌ Ongeldige identifier: $identifier"
                                return
                            }
                        } catch {
                            Write-Host "[$portName] ❌ Fout bij het bepalen van de categorie: $_"
                            return
                        }

                        $baudRate = 9600
                        $port = New-Object System.IO.Ports.SerialPort $portName, $baudRate, 'None', 8, 'One'
                        $port.ReadTimeout = 1000

                        try {
                            $port.Open()
                            Write-Host "✅ Thread gestart voor $portName ($identifier)"
                        } catch {
                            Write-Host "❌ Kon ${portName} niet openen voor ${identifier}: $_"
                            return
                        }

                        while ($true) {
                            try {
                                $line = $port.ReadLine().Trim()
                                Write-Host "[$portName] Ontvangen: $line"

                                if ($line -eq "BUTTON_PRESSED") {
                                    Write-Host "[$portName] 🟢 Request wordt verstuurd voor $identifier..."
                                    try {
                                        # Gebruik de dynamische URL met de categorie
                                        $requestUrl = [string]::Format($apiEndpoint, $category)
                                        Invoke-WebRequest -Uri $requestUrl -UseBasicParsing

                                        Write-Host "[$portName] ✅ Verzoek verstuurd om $(Get-Date -Format 'HH:mm:ss')"
                                        Start-Sleep -Seconds 5
                                    } catch {
                                        Write-Host "[$portName] ❌ Fout bij verzoek: $_"
                                    }
                                }
                            } catch {
                                Write-Host "[$portName] ⚠️ Leesfout: $_"
                                if (-not $port.IsOpen) {
                                    Write-Host "[$portName] 🔄 Poort is gesloten, stoppen met monitoren..."
                                    break
                                } else {
                                    Start-Sleep -Milliseconds 500
                                }
                            }
                        }
                    }
                    Monitor-Port -portName $portName -identifier $identifier
                } -ArgumentList $portName, $identifier, $useSystemWeb, $apiEndpoint
            }
        }

        # Wacht even voordat je opnieuw scant
        Start-Sleep -Seconds 5

        # Verwijder voltooide of gestopte jobs
        Get-Job | Where-Object { $_.State -ne "Running" } | Remove-Job
    }
} catch {
    Write-Host "❌ Onverwachte fout in hoofdloop: $_"
    Write-Host "Druk op een toets om af te sluiten..."
    $null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")
    exit
}