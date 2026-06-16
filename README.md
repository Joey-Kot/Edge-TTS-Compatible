# Edge-TTS-Compatible

This is an OpenAI-compatible Text-to-Speech service backed directly by Microsoft Edge TTS. It implements the reverse-engineered Edge TTS HTTP/WebSocket protocol in Go and exposes a small API surface compatible with OpenAI-style speech clients.

The public model name is always `edge`. The server accepts the client-provided `model` field for compatibility, but ignores its value when calling the Edge TTS upstream.

It is configured with command-line flags for local runs and environment variables for container deployment.

## Configuration and Usage

For local binary runs:

```bash
go build -trimpath -ldflags="-s -w" -o edge-tts-compatible ./cmd/server
```

```bash
./edge-tts-compatible \
  --listen :8080 \
  --api-token sk-local \
  --default-voice en-US-EmmaMultilingualNeural \
  --upstream-timeout 120 \
  --upstream-concurrency 10 \
  --upstream-interval-ms 500
```

Create raw MP3 speech:

```bash
curl http://localhost:8080/v1/audio/speech \
  -H 'Authorization: Bearer sk-local' \
  -H 'Content-Type: application/json' \
  -d '{"model":"edge","input":"Hello world.","voice":"en-US-EmmaMultilingualNeural"}' \
  --output speech.mp3
```

Create streaming SSE speech:

```bash
curl http://localhost:8080/v1/audio/speech \
  -H 'Authorization: Bearer sk-local' \
  -H 'Content-Type: application/json' \
  -d '{"model":"edge","input":"Hello world.","voice":"en-US-EmmaMultilingualNeural","stream_format":"sse"}'
```

For container deployment, use `docker.env.example`:

```bash
cp docker.env.example docker.env
docker build -t edge-tts-compatible:latest .
docker run -itd \
  --name edge-tts-compatible \
  -p 8080:8080 \
  --env-file docker.env \
  --restart always \
  edge-tts-compatible:latest
```

Container environment reference:

| Environment variable | Equivalent flag |
| --- | --- |
| `LISTEN` | `--listen` |
| `API_TOKEN` | `--api-token` |
| `DEFAULT_VOICE` | `--default-voice` |
| `PROXY` | `--proxy` |
| `UPSTREAM_TIMEOUT` | `--upstream-timeout` |
| `UPSTREAM_CONCURRENCY` | `--upstream-concurrency` |
| `UPSTREAM_INTERVAL_MS` | `--upstream-interval-ms` |
| `READ_HEADER_TIMEOUT` | `--read-header-timeout` |
| `IDLE_TIMEOUT` | `--idle-timeout` |

If `--api-token` is empty, local authentication is disabled. When configured, the API token is accepted through `Authorization: Bearer ...` or `x-api-key`.

## Compatible Endpoints

### OpenAI Audio Speech

| Endpoint | Description |
| --- | --- |
| `POST /v1/audio/speech` | Create OpenAI-compatible speech audio through Edge TTS. |
| `POST /audio/speech` | Alias for `/v1/audio/speech`. |

### Models and Voices

| Endpoint | Description |
| --- | --- |
| `GET /v1/models` | Return a single local model entry: `edge`. |
| `GET /v1/voices` | Return the upstream Edge TTS voice list. |
| `POST /v1/voices` | Alias for voice listing. |
| `GET /voices` | Alias for voice listing. |
| `POST /voices` | Alias for voice listing. |
| `GET /v1/audio/voices` | Alias for voice listing. |
| `POST /v1/audio/voices` | Alias for voice listing. |
| `GET /audio/voices` | Alias for voice listing. |
| `POST /audio/voices` | Alias for voice listing. |
| `GET /health` | Health check endpoint. |

## Request Parameters

| Parameter | Description |
| --- | --- |
| `model` | Accepted for OpenAI compatibility. The value is ignored; the server always uses Edge TTS. |
| `input` | Required text input. Maximum length is 4096 characters. |
| `voice` | Edge voice short name, or an object like `{ "id": "en-US-EmmaMultilingualNeural" }`. |
| `response_format` | Only `mp3` is supported because the real Edge upstream emits MP3. |
| `stream_format` | `audio` for raw MP3 bytes, or `sse` for base64 audio delta events. Defaults to `audio`. |
| `speed` | OpenAI-style speed multiplier or Edge rate string. `1.0` maps to `+0%`, `1.5` maps to `+50%`, `0.5` maps to `-50%`. |
| `volume` | Edge prosody volume, for example `+0%` or `-50%`. Defaults to `+0%`. |
| `pitch` | Edge prosody pitch, for example `+0Hz` or `-50Hz`. Defaults to `+0Hz`. |

Example request:

```json
{
  "model": "anything",
  "input": "Hello world.",
  "voice": "en-US-EmmaMultilingualNeural",
  "response_format": "mp3",
  "stream_format": "audio",
  "speed": 1.0,
  "volume": "+0%",
  "pitch": "+0Hz"
}
```

## Mapping Notes

| Target feature | Edge TTS mapping |
| --- | --- |
| OpenAI model | Exposes `edge`; incoming request model values are ignored. |
| `voice` | Forwarded as the Edge TTS voice short name. |
| `speed` | Converted to Edge prosody `rate`. |
| `volume` | Forwarded as Edge prosody `volume`. |
| `pitch` | Forwarded as Edge prosody `pitch`. |
| Non-streaming audio | Streams upstream MP3 chunks directly to the HTTP response. |
| SSE streaming | Emits `speech.audio.delta` events with base64 audio and a final `speech.audio.done` event. |
| Upstream concurrency | Client requests wait in an internal queue when the Edge TTS concurrency limit is reached. |
| Upstream interval | A global minimum delay is enforced between any two Edge TTS upstream requests. |

Default upstream throttling is `10` concurrent Edge TTS requests and `500ms` between upstream requests. Override it with `--upstream-concurrency` and `--upstream-interval-ms`.

## Supported Voices

| ShortName | Gender | Locale | FriendlyName |
|---|---|---|---|
| af-ZA-AdriNeural | Female | af-ZA | Microsoft Adri Online (Natural) - Afrikaans (South Africa) |
| af-ZA-WillemNeural | Male | af-ZA | Microsoft Willem Online (Natural) - Afrikaans (South Africa) |
| sq-AL-AnilaNeural | Female | sq-AL | Microsoft Anila Online (Natural) - Albanian (Albania) |
| sq-AL-IlirNeural | Male | sq-AL | Microsoft Ilir Online (Natural) - Albanian (Albania) |
| am-ET-AmehaNeural | Male | am-ET | Microsoft Ameha Online (Natural) - Amharic (Ethiopia) |
| am-ET-MekdesNeural | Female | am-ET | Microsoft Mekdes Online (Natural) - Amharic (Ethiopia) |
| ar-DZ-AminaNeural | Female | ar-DZ | Microsoft Amina Online (Natural) - Arabic (Algeria) |
| ar-DZ-IsmaelNeural | Male | ar-DZ | Microsoft Ismael Online (Natural) - Arabic (Algeria) |
| ar-BH-AliNeural | Male | ar-BH | Microsoft Ali Online (Natural) - Arabic (Bahrain) |
| ar-BH-LailaNeural | Female | ar-BH | Microsoft Laila Online (Natural) - Arabic (Bahrain) |
| ar-EG-SalmaNeural | Female | ar-EG | Microsoft Salma Online (Natural) - Arabic (Egypt) |
| ar-EG-ShakirNeural | Male | ar-EG | Microsoft Shakir Online (Natural) - Arabic (Egypt) |
| ar-IQ-BasselNeural | Male | ar-IQ | Microsoft Bassel Online (Natural) - Arabic (Iraq) |
| ar-IQ-RanaNeural | Female | ar-IQ | Microsoft Rana Online (Natural) - Arabic (Iraq) |
| ar-JO-SanaNeural | Female | ar-JO | Microsoft Sana Online (Natural) - Arabic (Jordan) |
| ar-JO-TaimNeural | Male | ar-JO | Microsoft Taim Online (Natural) - Arabic (Jordan) |
| ar-KW-FahedNeural | Male | ar-KW | Microsoft Fahed Online (Natural) - Arabic (Kuwait) |
| ar-KW-NouraNeural | Female | ar-KW | Microsoft Noura Online (Natural) - Arabic (Kuwait) |
| ar-LB-LaylaNeural | Female | ar-LB | Microsoft Layla Online (Natural) - Arabic (Lebanon) |
| ar-LB-RamiNeural | Male | ar-LB | Microsoft Rami Online (Natural) - Arabic (Lebanon) |
| ar-LY-ImanNeural | Female | ar-LY | Microsoft Iman Online (Natural) - Arabic (Libya) |
| ar-LY-OmarNeural | Male | ar-LY | Microsoft Omar Online (Natural) - Arabic (Libya) |
| ar-MA-JamalNeural | Male | ar-MA | Microsoft Jamal Online (Natural) - Arabic (Morocco) |
| ar-MA-MounaNeural | Female | ar-MA | Microsoft Mouna Online (Natural) - Arabic (Morocco) |
| ar-OM-AbdullahNeural | Male | ar-OM | Microsoft Abdullah Online (Natural) - Arabic (Oman) |
| ar-OM-AyshaNeural | Female | ar-OM | Microsoft Aysha Online (Natural) - Arabic (Oman) |
| ar-QA-AmalNeural | Female | ar-QA | Microsoft Amal Online (Natural) - Arabic (Qatar) |
| ar-QA-MoazNeural | Male | ar-QA | Microsoft Moaz Online (Natural) - Arabic (Qatar) |
| ar-SA-HamedNeural | Male | ar-SA | Microsoft Hamed Online (Natural) - Arabic (Saudi Arabia) |
| ar-SA-ZariyahNeural | Female | ar-SA | Microsoft Zariyah Online (Natural) - Arabic (Saudi Arabia) |
| ar-SY-AmanyNeural | Female | ar-SY | Microsoft Amany Online (Natural) - Arabic (Syria) |
| ar-SY-LaithNeural | Male | ar-SY | Microsoft Laith Online (Natural) - Arabic (Syria) |
| ar-TN-HediNeural | Male | ar-TN | Microsoft Hedi Online (Natural) - Arabic (Tunisia) |
| ar-TN-ReemNeural | Female | ar-TN | Microsoft Reem Online (Natural) - Arabic (Tunisia) |
| ar-AE-FatimaNeural | Female | ar-AE | Microsoft Fatima Online (Natural) - Arabic (United Arab Emirates) |
| ar-AE-HamdanNeural | Male | ar-AE | Microsoft Hamdan Online (Natural) - Arabic (United Arab Emirates) |
| ar-YE-MaryamNeural | Female | ar-YE | Microsoft Maryam Online (Natural) - Arabic (Yemen) |
| ar-YE-SalehNeural | Male | ar-YE | Microsoft Saleh Online (Natural) - Arabic (Yemen) |
| az-AZ-BabekNeural | Male | az-AZ | Microsoft Babek Online (Natural) - Azerbaijani (Azerbaijan) |
| az-AZ-BanuNeural | Female | az-AZ | Microsoft Banu Online (Natural) - Azerbaijani (Azerbaijan) |
| bn-BD-NabanitaNeural | Female | bn-BD | Microsoft Nabanita Online (Natural) - Bangla (Bangladesh) |
| bn-BD-PradeepNeural | Male | bn-BD | Microsoft Pradeep Online (Natural) - Bangla (Bangladesh) |
| bn-IN-BashkarNeural | Male | bn-IN | Microsoft Bashkar Online (Natural) - Bangla (India) |
| bn-IN-TanishaaNeural | Female | bn-IN | Microsoft Tanishaa Online (Natural) - Bengali (India) |
| bs-BA-VesnaNeural | Female | bs-BA | Microsoft Vesna Online (Natural) - Bosnian (Bosnia and Herzegovina) |
| bs-BA-GoranNeural | Male | bs-BA | Microsoft Goran Online (Natural) - Bosnian (Bosnia) |
| bg-BG-BorislavNeural | Male | bg-BG | Microsoft Borislav Online (Natural) - Bulgarian (Bulgaria) |
| bg-BG-KalinaNeural | Female | bg-BG | Microsoft Kalina Online (Natural) - Bulgarian (Bulgaria) |
| my-MM-NilarNeural | Female | my-MM | Microsoft Nilar Online (Natural) - Burmese (Myanmar) |
| my-MM-ThihaNeural | Male | my-MM | Microsoft Thiha Online (Natural) - Burmese (Myanmar) |
| ca-ES-EnricNeural | Male | ca-ES | Microsoft Enric Online (Natural) - Catalan |
| ca-ES-JoanaNeural | Female | ca-ES | Microsoft Joana Online (Natural) - Catalan |
| zh-HK-HiuGaaiNeural | Female | zh-HK | Microsoft HiuGaai Online (Natural) - Chinese (Cantonese Traditional) |
| zh-HK-HiuMaanNeural | Female | zh-HK | Microsoft HiuMaan Online (Natural) - Chinese (Hong Kong SAR) |
| zh-HK-WanLungNeural | Male | zh-HK | Microsoft WanLung Online (Natural) - Chinese (Hong Kong SAR) |
| zh-CN-XiaoxiaoNeural | Female | zh-CN | Microsoft Xiaoxiao Online (Natural) - Chinese (Mainland) |
| zh-CN-XiaoyiNeural | Female | zh-CN | Microsoft Xiaoyi Online (Natural) - Chinese (Mainland) |
| zh-CN-YunjianNeural | Male | zh-CN | Microsoft Yunjian Online (Natural) - Chinese (Mainland) |
| zh-CN-YunxiNeural | Male | zh-CN | Microsoft Yunxi Online (Natural) - Chinese (Mainland) |
| zh-CN-YunxiaNeural | Male | zh-CN | Microsoft Yunxia Online (Natural) - Chinese (Mainland) |
| zh-CN-YunyangNeural | Male | zh-CN | Microsoft Yunyang Online (Natural) - Chinese (Mainland) |
| zh-CN-liaoning-XiaobeiNeural | Female | zh-CN-liaoning | Microsoft Xiaobei Online (Natural) - Chinese (Northeastern Mandarin) |
| zh-TW-HsiaoChenNeural | Female | zh-TW | Microsoft HsiaoChen Online (Natural) - Chinese (Taiwan) |
| zh-TW-YunJheNeural | Male | zh-TW | Microsoft YunJhe Online (Natural) - Chinese (Taiwan) |
| zh-TW-HsiaoYuNeural | Female | zh-TW | Microsoft HsiaoYu Online (Natural) - Chinese (Taiwanese Mandarin) |
| zh-CN-shaanxi-XiaoniNeural | Female | zh-CN-shaanxi | Microsoft Xiaoni Online (Natural) - Chinese (Zhongyuan Mandarin Shaanxi) |
| hr-HR-GabrijelaNeural | Female | hr-HR | Microsoft Gabrijela Online (Natural) - Croatian (Croatia) |
| hr-HR-SreckoNeural | Male | hr-HR | Microsoft Srecko Online (Natural) - Croatian (Croatia) |
| cs-CZ-AntoninNeural | Male | cs-CZ | Microsoft Antonin Online (Natural) - Czech (Czech) |
| cs-CZ-VlastaNeural | Female | cs-CZ | Microsoft Vlasta Online (Natural) - Czech (Czech) |
| da-DK-ChristelNeural | Female | da-DK | Microsoft Christel Online (Natural) - Danish (Denmark) |
| da-DK-JeppeNeural | Male | da-DK | Microsoft Jeppe Online (Natural) - Danish (Denmark) |
| nl-BE-ArnaudNeural | Male | nl-BE | Microsoft Arnaud Online (Natural) - Dutch (Belgium) |
| nl-BE-DenaNeural | Female | nl-BE | Microsoft Dena Online (Natural) - Dutch (Belgium) |
| nl-NL-ColetteNeural | Female | nl-NL | Microsoft Colette Online (Natural) - Dutch (Netherlands) |
| nl-NL-FennaNeural | Female | nl-NL | Microsoft Fenna Online (Natural) - Dutch (Netherlands) |
| nl-NL-MaartenNeural | Male | nl-NL | Microsoft Maarten Online (Natural) - Dutch (Netherlands) |
| en-AU-WilliamMultilingualNeural | Male | en-AU | Microsoft WilliamMultilingual Online (Natural) - English (Australia) |
| en-AU-NatashaNeural | Female | en-AU | Microsoft Natasha Online (Natural) - English (Australia) |
| en-CA-ClaraNeural | Female | en-CA | Microsoft Clara Online (Natural) - English (Canada) |
| en-CA-LiamNeural | Male | en-CA | Microsoft Liam Online (Natural) - English (Canada) |
| en-HK-YanNeural | Female | en-HK | Microsoft Yan Online (Natural) - English (Hong Kong SAR) |
| en-HK-SamNeural | Male | en-HK | Microsoft Sam Online (Natural) - English (Hongkong) |
| en-IN-NeerjaExpressiveNeural | Female | en-IN | Microsoft Neerja Online (Natural) - English (India) (Preview) |
| en-IN-NeerjaNeural | Female | en-IN | Microsoft Neerja Online (Natural) - English (India) |
| en-IN-PrabhatNeural | Male | en-IN | Microsoft Prabhat Online (Natural) - English (India) |
| en-IE-ConnorNeural | Male | en-IE | Microsoft Connor Online (Natural) - English (Ireland) |
| en-IE-EmilyNeural | Female | en-IE | Microsoft Emily Online (Natural) - English (Ireland) |
| en-KE-AsiliaNeural | Female | en-KE | Microsoft Asilia Online (Natural) - English (Kenya) |
| en-KE-ChilembaNeural | Male | en-KE | Microsoft Chilemba Online (Natural) - English (Kenya) |
| en-NZ-MitchellNeural | Male | en-NZ | Microsoft Mitchell Online (Natural) - English (New Zealand) |
| en-NZ-MollyNeural | Female | en-NZ | Microsoft Molly Online (Natural) - English (New Zealand) |
| en-NG-AbeoNeural | Male | en-NG | Microsoft Abeo Online (Natural) - English (Nigeria) |
| en-NG-EzinneNeural | Female | en-NG | Microsoft Ezinne Online (Natural) - English (Nigeria) |
| en-PH-JamesNeural | Male | en-PH | Microsoft James Online (Natural) - English (Philippines) |
| en-PH-RosaNeural | Female | en-PH | Microsoft Rosa Online (Natural) - English (Philippines) |
| en-US-AvaNeural | Female | en-US | Microsoft Ava Online (Natural) - English (United States) |
| en-US-AndrewNeural | Male | en-US | Microsoft Andrew Online (Natural) - English (United States) |
| en-US-EmmaNeural | Female | en-US | Microsoft Emma Online (Natural) - English (United States) |
| en-US-BrianNeural | Male | en-US | Microsoft Brian Online (Natural) - English (United States) |
| en-SG-LunaNeural | Female | en-SG | Microsoft Luna Online (Natural) - English (Singapore) |
| en-SG-WayneNeural | Male | en-SG | Microsoft Wayne Online (Natural) - English (Singapore) |
| en-ZA-LeahNeural | Female | en-ZA | Microsoft Leah Online (Natural) - English (South Africa) |
| en-ZA-LukeNeural | Male | en-ZA | Microsoft Luke Online (Natural) - English (South Africa) |
| en-TZ-ElimuNeural | Male | en-TZ | Microsoft Elimu Online (Natural) - English (Tanzania) |
| en-TZ-ImaniNeural | Female | en-TZ | Microsoft Imani Online (Natural) - English (Tanzania) |
| en-GB-LibbyNeural | Female | en-GB | Microsoft Libby Online (Natural) - English (United Kingdom) |
| en-GB-MaisieNeural | Female | en-GB | Microsoft Maisie Online (Natural) - English (United Kingdom) |
| en-GB-RyanNeural | Male | en-GB | Microsoft Ryan Online (Natural) - English (United Kingdom) |
| en-GB-SoniaNeural | Female | en-GB | Microsoft Sonia Online (Natural) - English (United Kingdom) |
| en-GB-ThomasNeural | Male | en-GB | Microsoft Thomas Online (Natural) - English (United Kingdom) |
| en-US-AnaNeural | Female | en-US | Microsoft Ana Online (Natural) - English (United States) |
| en-US-AndrewMultilingualNeural | Male | en-US | Microsoft AndrewMultilingual Online (Natural) - English (United States) |
| en-US-AriaNeural | Female | en-US | Microsoft Aria Online (Natural) - English (United States) |
| en-US-AvaMultilingualNeural | Female | en-US | Microsoft AvaMultilingual Online (Natural) - English (United States) |
| en-US-BrianMultilingualNeural | Male | en-US | Microsoft BrianMultilingual Online (Natural) - English (United States) |
| en-US-ChristopherNeural | Male | en-US | Microsoft Christopher Online (Natural) - English (United States) |
| en-US-EmmaMultilingualNeural | Female | en-US | Microsoft EmmaMultilingual Online (Natural) - English (United States) |
| en-US-EricNeural | Male | en-US | Microsoft Eric Online (Natural) - English (United States) |
| en-US-GuyNeural | Male | en-US | Microsoft Guy Online (Natural) - English (United States) |
| en-US-JennyNeural | Female | en-US | Microsoft Jenny Online (Natural) - English (United States) |
| en-US-MichelleNeural | Female | en-US | Microsoft Michelle Online (Natural) - English (United States) |
| en-US-RogerNeural | Male | en-US | Microsoft Roger Online (Natural) - English (United States) |
| en-US-SteffanNeural | Male | en-US | Microsoft Steffan Online (Natural) - English (United States) |
| et-EE-AnuNeural | Female | et-EE | Microsoft Anu Online (Natural) - Estonian (Estonia) |
| et-EE-KertNeural | Male | et-EE | Microsoft Kert Online (Natural) - Estonian (Estonia) |
| fil-PH-AngeloNeural | Male | fil-PH | Microsoft Angelo Online (Natural) - Filipino (Philippines) |
| fil-PH-BlessicaNeural | Female | fil-PH | Microsoft Blessica Online (Natural) - Filipino (Philippines) |
| fi-FI-HarriNeural | Male | fi-FI | Microsoft Harri Online (Natural) - Finnish (Finland) |
| fi-FI-NooraNeural | Female | fi-FI | Microsoft Noora Online (Natural) - Finnish (Finland) |
| fr-BE-CharlineNeural | Female | fr-BE | Microsoft Charline Online (Natural) - French (Belgium) |
| fr-BE-GerardNeural | Male | fr-BE | Microsoft Gerard Online (Natural) - French (Belgium) |
| fr-CA-ThierryNeural | Male | fr-CA | Microsoft Thierry Online (Natural) - French (Canada) |
| fr-CA-AntoineNeural | Male | fr-CA | Microsoft Antoine Online (Natural) - French (Canada) |
| fr-CA-JeanNeural | Male | fr-CA | Microsoft Jean Online (Natural) - French (Canada) |
| fr-CA-SylvieNeural | Female | fr-CA | Microsoft Sylvie Online (Natural) - French (Canada) |
| fr-FR-VivienneMultilingualNeural | Female | fr-FR | Microsoft VivienneMultilingual Online (Natural) - French (France) |
| fr-FR-RemyMultilingualNeural | Male | fr-FR | Microsoft RemyMultilingual Online (Natural) - French (France) |
| fr-FR-DeniseNeural | Female | fr-FR | Microsoft Denise Online (Natural) - French (France) |
| fr-FR-EloiseNeural | Female | fr-FR | Microsoft Eloise Online (Natural) - French (France) |
| fr-FR-HenriNeural | Male | fr-FR | Microsoft Henri Online (Natural) - French (France) |
| fr-CH-ArianeNeural | Female | fr-CH | Microsoft Ariane Online (Natural) - French (Switzerland) |
| fr-CH-FabriceNeural | Male | fr-CH | Microsoft Fabrice Online (Natural) - French (Switzerland) |
| gl-ES-RoiNeural | Male | gl-ES | Microsoft Roi Online (Natural) - Galician |
| gl-ES-SabelaNeural | Female | gl-ES | Microsoft Sabela Online (Natural) - Galician |
| ka-GE-EkaNeural | Female | ka-GE | Microsoft Eka Online (Natural) - Georgian (Georgia) |
| ka-GE-GiorgiNeural | Male | ka-GE | Microsoft Giorgi Online (Natural) - Georgian (Georgia) |
| de-AT-IngridNeural | Female | de-AT | Microsoft Ingrid Online (Natural) - German (Austria) |
| de-AT-JonasNeural | Male | de-AT | Microsoft Jonas Online (Natural) - German (Austria) |
| de-DE-SeraphinaMultilingualNeural | Female | de-DE | Microsoft SeraphinaMultilingual Online (Natural) - German (Germany) |
| de-DE-FlorianMultilingualNeural | Male | de-DE | Microsoft FlorianMultilingual Online (Natural) - German (Germany) |
| de-DE-AmalaNeural | Female | de-DE | Microsoft Amala Online (Natural) - German (Germany) |
| de-DE-ConradNeural | Male | de-DE | Microsoft Conrad Online (Natural) - German (Germany) |
| de-DE-KatjaNeural | Female | de-DE | Microsoft Katja Online (Natural) - German (Germany) |
| de-DE-KillianNeural | Male | de-DE | Microsoft Killian Online (Natural) - German (Germany) |
| de-CH-JanNeural | Male | de-CH | Microsoft Jan Online (Natural) - German (Switzerland) |
| de-CH-LeniNeural | Female | de-CH | Microsoft Leni Online (Natural) - German (Switzerland) |
| el-GR-AthinaNeural | Female | el-GR | Microsoft Athina Online (Natural) - Greek (Greece) |
| el-GR-NestorasNeural | Male | el-GR | Microsoft Nestoras Online (Natural) - Greek (Greece) |
| gu-IN-DhwaniNeural | Female | gu-IN | Microsoft Dhwani Online (Natural) - Gujarati (India) |
| gu-IN-NiranjanNeural | Male | gu-IN | Microsoft Niranjan Online (Natural) - Gujarati (India) |
| he-IL-AvriNeural | Male | he-IL | Microsoft Avri Online (Natural) - Hebrew (Israel) |
| he-IL-HilaNeural | Female | he-IL | Microsoft Hila Online (Natural) - Hebrew (Israel) |
| hi-IN-MadhurNeural | Male | hi-IN | Microsoft Madhur Online (Natural) - Hindi (India) |
| hi-IN-SwaraNeural | Female | hi-IN | Microsoft Swara Online (Natural) - Hindi (India) |
| hu-HU-NoemiNeural | Female | hu-HU | Microsoft Noemi Online (Natural) - Hungarian (Hungary) |
| hu-HU-TamasNeural | Male | hu-HU | Microsoft Tamas Online (Natural) - Hungarian (Hungary) |
| is-IS-GudrunNeural | Female | is-IS | Microsoft Gudrun Online (Natural) - Icelandic (Iceland) |
| is-IS-GunnarNeural | Male | is-IS | Microsoft Gunnar Online (Natural) - Icelandic (Iceland) |
| id-ID-ArdiNeural | Male | id-ID | Microsoft Ardi Online (Natural) - Indonesian (Indonesia) |
| id-ID-GadisNeural | Female | id-ID | Microsoft Gadis Online (Natural) - Indonesian (Indonesia) |
| iu-Latn-CA-SiqiniqNeural | Female | iu-Latn-CA | Microsoft Siqiniq Online (Natural) - Inuktitut (Latin, Canada) |
| iu-Latn-CA-TaqqiqNeural | Male | iu-Latn-CA | Microsoft Taqqiq Online (Natural) - Inuktitut (Latin, Canada) |
| iu-Cans-CA-SiqiniqNeural | Female | iu-Cans-CA | Microsoft Siqiniq Online (Natural) - Inuktitut (Syllabics, Canada) |
| iu-Cans-CA-TaqqiqNeural | Male | iu-Cans-CA | Microsoft Taqqiq Online (Natural) - Inuktitut (Syllabics, Canada) |
| ga-IE-ColmNeural | Male | ga-IE | Microsoft Colm Online (Natural) - Irish (Ireland) |
| ga-IE-OrlaNeural | Female | ga-IE | Microsoft Orla Online (Natural) - Irish (Ireland) |
| it-IT-GiuseppeMultilingualNeural | Male | it-IT | Microsoft GiuseppeMultilingual Online (Natural) - Italian (Italy) |
| it-IT-DiegoNeural | Male | it-IT | Microsoft Diego Online (Natural) - Italian (Italy) |
| it-IT-ElsaNeural | Female | it-IT | Microsoft Elsa Online (Natural) - Italian (Italy) |
| it-IT-IsabellaNeural | Female | it-IT | Microsoft Isabella Online (Natural) - Italian (Italy) |
| ja-JP-KeitaNeural | Male | ja-JP | Microsoft Keita Online (Natural) - Japanese (Japan) |
| ja-JP-NanamiNeural | Female | ja-JP | Microsoft Nanami Online (Natural) - Japanese (Japan) |
| jv-ID-DimasNeural | Male | jv-ID | Microsoft Dimas Online (Natural) - Javanese (Indonesia) |
| jv-ID-SitiNeural | Female | jv-ID | Microsoft Siti Online (Natural) - Javanese (Indonesia) |
| kn-IN-GaganNeural | Male | kn-IN | Microsoft Gagan Online (Natural) - Kannada (India) |
| kn-IN-SapnaNeural | Female | kn-IN | Microsoft Sapna Online (Natural) - Kannada (India) |
| kk-KZ-AigulNeural | Female | kk-KZ | Microsoft Aigul Online (Natural) - Kazakh (Kazakhstan) |
| kk-KZ-DauletNeural | Male | kk-KZ | Microsoft Daulet Online (Natural) - Kazakh (Kazakhstan) |
| km-KH-PisethNeural | Male | km-KH | Microsoft Piseth Online (Natural) - Khmer (Cambodia) |
| km-KH-SreymomNeural | Female | km-KH | Microsoft Sreymom Online (Natural) - Khmer (Cambodia) |
| ko-KR-HyunsuMultilingualNeural | Male | ko-KR | Microsoft HyunsuMultilingual Online (Natural) - Korean (Korea) |
| ko-KR-InJoonNeural | Male | ko-KR | Microsoft InJoon Online (Natural) - Korean (Korea) |
| ko-KR-SunHiNeural | Female | ko-KR | Microsoft SunHi Online (Natural) - Korean (Korea) |
| lo-LA-ChanthavongNeural | Male | lo-LA | Microsoft Chanthavong Online (Natural) - Lao (Laos) |
| lo-LA-KeomanyNeural | Female | lo-LA | Microsoft Keomany Online (Natural) - Lao (Laos) |
| lv-LV-EveritaNeural | Female | lv-LV | Microsoft Everita Online (Natural) - Latvian (Latvia) |
| lv-LV-NilsNeural | Male | lv-LV | Microsoft Nils Online (Natural) - Latvian (Latvia) |
| lt-LT-LeonasNeural | Male | lt-LT | Microsoft Leonas Online (Natural) - Lithuanian (Lithuania) |
| lt-LT-OnaNeural | Female | lt-LT | Microsoft Ona Online (Natural) - Lithuanian (Lithuania) |
| mk-MK-AleksandarNeural | Male | mk-MK | Microsoft Aleksandar Online (Natural) - Macedonian (North Macedonia) |
| mk-MK-MarijaNeural | Female | mk-MK | Microsoft Marija Online (Natural) - Macedonian (North Macedonia) |
| ms-MY-OsmanNeural | Male | ms-MY | Microsoft Osman Online (Natural) - Malay (Malaysia) |
| ms-MY-YasminNeural | Female | ms-MY | Microsoft Yasmin Online (Natural) - Malay (Malaysia) |
| ml-IN-MidhunNeural | Male | ml-IN | Microsoft Midhun Online (Natural) - Malayalam (India) |
| ml-IN-SobhanaNeural | Female | ml-IN | Microsoft Sobhana Online (Natural) - Malayalam (India) |
| mt-MT-GraceNeural | Female | mt-MT | Microsoft Grace Online (Natural) - Maltese (Malta) |
| mt-MT-JosephNeural | Male | mt-MT | Microsoft Joseph Online (Natural) - Maltese (Malta) |
| mr-IN-AarohiNeural | Female | mr-IN | Microsoft Aarohi Online (Natural) - Marathi (India) |
| mr-IN-ManoharNeural | Male | mr-IN | Microsoft Manohar Online (Natural) - Marathi (India) |
| mn-MN-BataaNeural | Male | mn-MN | Microsoft Bataa Online (Natural) - Mongolian (Mongolia) |
| mn-MN-YesuiNeural | Female | mn-MN | Microsoft Yesui Online (Natural) - Mongolian (Mongolia) |
| ne-NP-HemkalaNeural | Female | ne-NP | Microsoft Hemkala Online (Natural) - Nepali (Nepal) |
| ne-NP-SagarNeural | Male | ne-NP | Microsoft Sagar Online (Natural) - Nepali (Nepal) |
| nb-NO-FinnNeural | Male | nb-NO | Microsoft Finn Online (Natural) - Norwegian (Bokmål Norway) |
| nb-NO-PernilleNeural | Female | nb-NO | Microsoft Pernille Online (Natural) - Norwegian (Bokmål, Norway) |
| ps-AF-GulNawazNeural | Male | ps-AF | Microsoft GulNawaz Online (Natural) - Pashto (Afghanistan) |
| ps-AF-LatifaNeural | Female | ps-AF | Microsoft Latifa Online (Natural) - Pashto (Afghanistan) |
| fa-IR-DilaraNeural | Female | fa-IR | Microsoft Dilara Online (Natural) - Persian (Iran) |
| fa-IR-FaridNeural | Male | fa-IR | Microsoft Farid Online (Natural) - Persian (Iran) |
| pl-PL-MarekNeural | Male | pl-PL | Microsoft Marek Online (Natural) - Polish (Poland) |
| pl-PL-ZofiaNeural | Female | pl-PL | Microsoft Zofia Online (Natural) - Polish (Poland) |
| pt-BR-ThalitaMultilingualNeural | Female | pt-BR | Microsoft ThalitaMultilingual Online (Natural) - Portuguese (Brazil) |
| pt-BR-AntonioNeural | Male | pt-BR | Microsoft Antonio Online (Natural) - Portuguese (Brazil) |
| pt-BR-FranciscaNeural | Female | pt-BR | Microsoft Francisca Online (Natural) - Portuguese (Brazil) |
| pt-PT-DuarteNeural | Male | pt-PT | Microsoft Duarte Online (Natural) - Portuguese (Portugal) |
| pt-PT-RaquelNeural | Female | pt-PT | Microsoft Raquel Online (Natural) - Portuguese (Portugal) |
| ro-RO-AlinaNeural | Female | ro-RO | Microsoft Alina Online (Natural) - Romanian (Romania) |
| ro-RO-EmilNeural | Male | ro-RO | Microsoft Emil Online (Natural) - Romanian (Romania) |
| ru-RU-DmitryNeural | Male | ru-RU | Microsoft Dmitry Online (Natural) - Russian (Russia) |
| ru-RU-SvetlanaNeural | Female | ru-RU | Microsoft Svetlana Online (Natural) - Russian (Russia) |
| sr-RS-NicholasNeural | Male | sr-RS | Microsoft Nicholas Online (Natural) - Serbian (Serbia) |
| sr-RS-SophieNeural | Female | sr-RS | Microsoft Sophie Online (Natural) - Serbian (Serbia) |
| si-LK-SameeraNeural | Male | si-LK | Microsoft Sameera Online (Natural) - Sinhala (Sri Lanka) |
| si-LK-ThiliniNeural | Female | si-LK | Microsoft Thilini Online (Natural) - Sinhala (Sri Lanka) |
| sk-SK-LukasNeural | Male | sk-SK | Microsoft Lukas Online (Natural) - Slovak (Slovakia) |
| sk-SK-ViktoriaNeural | Female | sk-SK | Microsoft Viktoria Online (Natural) - Slovak (Slovakia) |
| sl-SI-PetraNeural | Female | sl-SI | Microsoft Petra Online (Natural) - Slovenian (Slovenia) |
| sl-SI-RokNeural | Male | sl-SI | Microsoft Rok Online (Natural) - Slovenian (Slovenia) |
| so-SO-MuuseNeural | Male | so-SO | Microsoft Muuse Online (Natural) - Somali (Somalia) |
| so-SO-UbaxNeural | Female | so-SO | Microsoft Ubax Online (Natural) - Somali (Somalia) |
| es-AR-ElenaNeural | Female | es-AR | Microsoft Elena Online (Natural) - Spanish (Argentina) |
| es-AR-TomasNeural | Male | es-AR | Microsoft Tomas Online (Natural) - Spanish (Argentina) |
| es-BO-MarceloNeural | Male | es-BO | Microsoft Marcelo Online (Natural) - Spanish (Bolivia) |
| es-BO-SofiaNeural | Female | es-BO | Microsoft Sofia Online (Natural) - Spanish (Bolivia) |
| es-CL-CatalinaNeural | Female | es-CL | Microsoft Catalina Online (Natural) - Spanish (Chile) |
| es-CL-LorenzoNeural | Male | es-CL | Microsoft Lorenzo Online (Natural) - Spanish (Chile) |
| es-CO-GonzaloNeural | Male | es-CO | Microsoft Gonzalo Online (Natural) - Spanish (Colombia) |
| es-CO-SalomeNeural | Female | es-CO | Microsoft Salome Online (Natural) - Spanish (Colombia) |
| es-ES-XimenaNeural | Female | es-ES | Microsoft Ximena Online (Natural) - Spanish (Colombia) |
| es-CR-JuanNeural | Male | es-CR | Microsoft Juan Online (Natural) - Spanish (Costa Rica) |
| es-CR-MariaNeural | Female | es-CR | Microsoft Maria Online (Natural) - Spanish (Costa Rica) |
| es-CU-BelkysNeural | Female | es-CU | Microsoft Belkys Online (Natural) - Spanish (Cuba) |
| es-CU-ManuelNeural | Male | es-CU | Microsoft Manuel Online (Natural) - Spanish (Cuba) |
| es-DO-EmilioNeural | Male | es-DO | Microsoft Emilio Online (Natural) - Spanish (Dominican Republic) |
| es-DO-RamonaNeural | Female | es-DO | Microsoft Ramona Online (Natural) - Spanish (Dominican Republic) |
| es-EC-AndreaNeural | Female | es-EC | Microsoft Andrea Online (Natural) - Spanish (Ecuador) |
| es-EC-LuisNeural | Male | es-EC | Microsoft Luis Online (Natural) - Spanish (Ecuador) |
| es-SV-LorenaNeural | Female | es-SV | Microsoft Lorena Online (Natural) - Spanish (El Salvador) |
| es-SV-RodrigoNeural | Male | es-SV | Microsoft Rodrigo Online (Natural) - Spanish (El Salvador) |
| es-GQ-JavierNeural | Male | es-GQ | Microsoft Javier Online (Natural) - Spanish (Equatorial Guinea) |
| es-GQ-TeresaNeural | Female | es-GQ | Microsoft Teresa Online (Natural) - Spanish (Equatorial Guinea) |
| es-GT-AndresNeural | Male | es-GT | Microsoft Andres Online (Natural) - Spanish (Guatemala) |
| es-GT-MartaNeural | Female | es-GT | Microsoft Marta Online (Natural) - Spanish (Guatemala) |
| es-HN-CarlosNeural | Male | es-HN | Microsoft Carlos Online (Natural) - Spanish (Honduras) |
| es-HN-KarlaNeural | Female | es-HN | Microsoft Karla Online (Natural) - Spanish (Honduras) |
| es-MX-DaliaNeural | Female | es-MX | Microsoft Dalia Online (Natural) - Spanish (Mexico) |
| es-MX-JorgeNeural | Male | es-MX | Microsoft Jorge Online (Natural) - Spanish (Mexico) |
| es-NI-FedericoNeural | Male | es-NI | Microsoft Federico Online (Natural) - Spanish (Nicaragua) |
| es-NI-YolandaNeural | Female | es-NI | Microsoft Yolanda Online (Natural) - Spanish (Nicaragua) |
| es-PA-MargaritaNeural | Female | es-PA | Microsoft Margarita Online (Natural) - Spanish (Panama) |
| es-PA-RobertoNeural | Male | es-PA | Microsoft Roberto Online (Natural) - Spanish (Panama) |
| es-PY-MarioNeural | Male | es-PY | Microsoft Mario Online (Natural) - Spanish (Paraguay) |
| es-PY-TaniaNeural | Female | es-PY | Microsoft Tania Online (Natural) - Spanish (Paraguay) |
| es-PE-AlexNeural | Male | es-PE | Microsoft Alex Online (Natural) - Spanish (Peru) |
| es-PE-CamilaNeural | Female | es-PE | Microsoft Camila Online (Natural) - Spanish (Peru) |
| es-PR-KarinaNeural | Female | es-PR | Microsoft Karina Online (Natural) - Spanish (Puerto Rico) |
| es-PR-VictorNeural | Male | es-PR | Microsoft Victor Online (Natural) - Spanish (Puerto Rico) |
| es-ES-AlvaroNeural | Male | es-ES | Microsoft Alvaro Online (Natural) - Spanish (Spain) |
| es-ES-ElviraNeural | Female | es-ES | Microsoft Elvira Online (Natural) - Spanish (Spain) |
| es-US-AlonsoNeural | Male | es-US | Microsoft Alonso Online (Natural) - Spanish (United States) |
| es-US-PalomaNeural | Female | es-US | Microsoft Paloma Online (Natural) - Spanish (United States) |
| es-UY-MateoNeural | Male | es-UY | Microsoft Mateo Online (Natural) - Spanish (Uruguay) |
| es-UY-ValentinaNeural | Female | es-UY | Microsoft Valentina Online (Natural) - Spanish (Uruguay) |
| es-VE-PaolaNeural | Female | es-VE | Microsoft Paola Online (Natural) - Spanish (Venezuela) |
| es-VE-SebastianNeural | Male | es-VE | Microsoft Sebastian Online (Natural) - Spanish (Venezuela) |
| su-ID-JajangNeural | Male | su-ID | Microsoft Jajang Online (Natural) - Sundanese (Indonesia) |
| su-ID-TutiNeural | Female | su-ID | Microsoft Tuti Online (Natural) - Sundanese (Indonesia) |
| sw-KE-RafikiNeural | Male | sw-KE | Microsoft Rafiki Online (Natural) - Swahili (Kenya) |
| sw-KE-ZuriNeural | Female | sw-KE | Microsoft Zuri Online (Natural) - Swahili (Kenya) |
| sw-TZ-DaudiNeural | Male | sw-TZ | Microsoft Daudi Online (Natural) - Swahili (Tanzania) |
| sw-TZ-RehemaNeural | Female | sw-TZ | Microsoft Rehema Online (Natural) - Swahili (Tanzania) |
| sv-SE-MattiasNeural | Male | sv-SE | Microsoft Mattias Online (Natural) - Swedish (Sweden) |
| sv-SE-SofieNeural | Female | sv-SE | Microsoft Sofie Online (Natural) - Swedish (Sweden) |
| ta-IN-PallaviNeural | Female | ta-IN | Microsoft Pallavi Online (Natural) - Tamil (India) |
| ta-IN-ValluvarNeural | Male | ta-IN | Microsoft Valluvar Online (Natural) - Tamil (India) |
| ta-MY-KaniNeural | Female | ta-MY | Microsoft Kani Online (Natural) - Tamil (Malaysia) |
| ta-MY-SuryaNeural | Male | ta-MY | Microsoft Surya Online (Natural) - Tamil (Malaysia) |
| ta-SG-AnbuNeural | Male | ta-SG | Microsoft Anbu Online (Natural) - Tamil (Singapore) |
| ta-SG-VenbaNeural | Female | ta-SG | Microsoft Venba Online (Natural) - Tamil (Singapore) |
| ta-LK-KumarNeural | Male | ta-LK | Microsoft Kumar Online (Natural) - Tamil (Sri Lanka) |
| ta-LK-SaranyaNeural | Female | ta-LK | Microsoft Saranya Online (Natural) - Tamil (Sri Lanka) |
| te-IN-MohanNeural | Male | te-IN | Microsoft Mohan Online (Natural) - Telugu (India) |
| te-IN-ShrutiNeural | Female | te-IN | Microsoft Shruti Online (Natural) - Telugu (India) |
| th-TH-NiwatNeural | Male | th-TH | Microsoft Niwat Online (Natural) - Thai (Thailand) |
| th-TH-PremwadeeNeural | Female | th-TH | Microsoft Premwadee Online (Natural) - Thai (Thailand) |
| tr-TR-EmelNeural | Female | tr-TR | Microsoft Emel Online (Natural) - Turkish (Turkey) |
| tr-TR-AhmetNeural | Male | tr-TR | Microsoft Ahmet Online (Natural) - Turkish (Türkiye) |
| uk-UA-OstapNeural | Male | uk-UA | Microsoft Ostap Online (Natural) - Ukrainian (Ukraine) |
| uk-UA-PolinaNeural | Female | uk-UA | Microsoft Polina Online (Natural) - Ukrainian (Ukraine) |
| ur-IN-GulNeural | Female | ur-IN | Microsoft Gul Online (Natural) - Urdu (India) |
| ur-IN-SalmanNeural | Male | ur-IN | Microsoft Salman Online (Natural) - Urdu (India) |
| ur-PK-AsadNeural | Male | ur-PK | Microsoft Asad Online (Natural) - Urdu (Pakistan) |
| ur-PK-UzmaNeural | Female | ur-PK | Microsoft Uzma Online (Natural) - Urdu (Pakistan) |
| uz-UZ-MadinaNeural | Female | uz-UZ | Microsoft Madina Online (Natural) - Uzbek (Uzbekistan) |
| uz-UZ-SardorNeural | Male | uz-UZ | Microsoft Sardor Online (Natural) - Uzbek (Uzbekistan) |
| vi-VN-HoaiMyNeural | Female | vi-VN | Microsoft HoaiMy Online (Natural) - Vietnamese (Vietnam) |
| vi-VN-NamMinhNeural | Male | vi-VN | Microsoft NamMinh Online (Natural) - Vietnamese (Vietnam) |
| cy-GB-AledNeural | Male | cy-GB | Microsoft Aled Online (Natural) - Welsh (United Kingdom) |
| cy-GB-NiaNeural | Female | cy-GB | Microsoft Nia Online (Natural) - Welsh (United Kingdom) |
| zu-ZA-ThandoNeural | Female | zu-ZA | Microsoft Thando Online (Natural) - Zulu (South Africa) |
| zu-ZA-ThembaNeural | Male | zu-ZA | Microsoft Themba Online (Natural) - Zulu (South Africa) |
