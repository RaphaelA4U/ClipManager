# ClipManager

Een eenvoudige, snelle en lichtgewicht applicatie om clips op te nemen van een RTSP-camera en te verzenden naar Telegram.

## Vereisten
- Docker en Docker Compose
- Een RTSP-camera (bijv. `rtsp://gebruiker:wachtwoord@camera-ip:poort/pad`)
- Een Telegram-bot token en chat ID

## Installatie
1. **Clone de repository**:
   ```bash
   git clone https://github.com/RaphaelA4U/ClipManager
   cd clipmanager
   ```

2. **Configureer de poort (optioneel)**: Kopieer `.env.example` naar `.env` en stel de poort in (standaard 8080):
   ```bash
   cp .env.example .env
   ```
   
   Bewerk `.env` indien nodig:
   ```
   PORT=8080
   ```

3. **Start de applicatie**:
   ```bash
   docker-compose up --build
   ```

4. **Controleer de logs**: Bij het opstarten zie je een bericht zoals:
   ```
   ClipManager gestart! Maak een GET/POST request naar localhost:8080/api/clip met parameters: camera_ip, chat_app, bot_token, chat_id, backtrack_seconds, duration_seconds
   ```

## Gebruik

Maak een GET- of POST-verzoek naar `localhost:8080/api/clip` met de volgende parameters:

### Parameters
| Parameter | Beschrijving | Voorbeeld | Verplicht |
|-----------|-------------|-----------|-----------|
| camera_ip | De RTSP-URL van de camera | rtsp://gebruiker:wachtwoord@camera-ip:poort/pad | Ja |
| chat_app | De chat-app (alleen Telegram ondersteund) | Telegram | Ja |
| bot_token | De Telegram bot token | 123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ | Ja |
| chat_id | De Telegram chat ID | -100123456789 | Ja |
| backtrack_seconds | Aantal seconden terug om op te nemen | 10 | Ja |
| duration_seconds | Duur van de clip in seconden | 10 | Ja |

### GET-voorbeeld:
```bash
curl "localhost:8080/api/clip?camera_ip=rtsp://gebruiker:wachtwoord@camera-ip:poort/pad&chat_app=Telegram&bot_token=123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ&chat_id=-100123456789&backtrack_seconds=10&duration_seconds=10"
```

### POST-voorbeeld:
```bash
curl -X POST localhost:8080/api/clip -H "Content-Type: application/json" -d '{"camera_ip":"rtsp://gebruiker:wachtwoord@camera-ip:poort/pad","chat_app":"Telegram","bot_token":"123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZ","chat_id":"-100123456789","backtrack_seconds":10,"duration_seconds":10}'
```

### Response

Bij succes:
```json
{"message":"Clip opgenomen en verzending gestart"}
```

Bij fouten ontvang je een HTTP-foutcode met een beschrijving.

## Opmerkingen

- De clip wordt lokaal opgeslagen in de `clips`-directory en na verzending verwijderd.
- Er wordt geen database gebruikt; de app is volledig stateless.
- De app is geoptimaliseerd voor snelheid en gebruikt een minimale Go-binary met FFmpeg.
- Voor maximale prestaties wordt de clip asynchroon verzonden naar Telegram.

## Probleemoplossing

- **FFmpeg-fouten**: Zorg ervoor dat de `camera_ip` correct is en dat de RTSP-stream toegankelijk is.
- **Telegram-fouten**: Controleer of de `bot_token` en `chat_id` correct zijn.
- **Logs**: Bekijk de Docker-logs voor meer informatie:
  ```bash
  docker-compose logs
  ```
