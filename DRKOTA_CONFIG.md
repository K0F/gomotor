# Drkota (GoMotor) — Kompletní technická konfigurace & specifikace

Vertikální V-plotter (polargraph) pro vektorové kreslení, typografii a sonifikaci telemetrie.

Dokument vygenerován: 2026-11-15  
Verze firmwaru a hosta: `v2.6-fleet-bazina`

---

## 1. Fyzická topologie & umístění v síti

| Parametr | Hodnota / Popis |
|:---|:---|
| **Fyzický uzel** | **`bazina`** (`192.168.1.104` / LAN `10.0.0.15`) |
| **Sériové rozhraní** | **`/dev/ttyUSB0`** |
| **Přenosová rychlost** | **`115200` baud**, 8N1 |
| **Udev pravidlo** | `/etc/udev/rules.d/99-drkota-serial.rules` (`ATTRS{idVendor}=="1a86", ATTRS{idProduct}=="7523"` / CH340 nebo FTDI) |
| **Síťový přístup** | Transparentní proxy přes SSH z `t520` a `boxC` (skript `~/bin/drkota`) |
| **Pracovní repozitář** | `~/src/gomotor` (zrcadleno na `t520`, `bazina`, `boxC:repos/gomotor.git`) |
| **Lokální LLM agent** | `drkota-qwen` napojený na `http://bazina:8080` (`Qwen3-8B-Q4_K_M`) |

---

## 2. Elektronika & Zapojení pinů (Pinout)

Drkota běží na desce **Arduino Uno (ATmega328P, 16 MHz)** s rozšiřujícím štítem **Arduino CNC Shield V3**.

### Krokové motory (NEMA 17 + Drivery A4988 / DRV8825)
- **Microstepping:** 1/16 (propojeny všechny 3 jumpery `MS1, MS2, MS3` pod driverem).
- **Logika řízení:**

| Osa / Funkce | Arduino Pin | Port ATmega328P | Popis na CNC Shieldu |
|:---|:---|:---|:---|
| **Motor A (Levý) STEP** | `Pin 2` | `PORTD2` | `X.STEP` |
| **Motor A (Levý) DIR** | `Pin 5` | `PORTD5` | `X.DIR` |
| **Motor B (Pravý) STEP** | `Pin 3` | `PORTD3` | `Y.STEP` |
| **Motor B (Pravý) DIR** | `Pin 6` | `PORTD6` | `Y.DIR` |
| **Společný ENABLE** | `Pin 8` | `PORTB0` | `EN` (aktivní v logické `LOW`) |

### Servo pera (Multi-Pin PWM architektura)
Kvůli kompatibilitě různých revizí CNC Shieldu V3 posílá firmware (`firmware/drkota_firmware/drkota_firmware.ino`) pulzy na **všech 8 kandidátních pinů současně**:

| Číslo pinu | Fyzické označení na CNC Shieldu | Výchozí funkce Grbl |
|:---|:---|:---|
| **Pin 9** | `X+` / `X-` | Limitní switch X |
| **Pin 10** | `Y+` / `Y-` | Limitní switch Y |
| **Pin 11** | `Z+` / `Z-` | Spindle Enable (Grbl v0.9+) |
| **Pin 12** | `SpnEn` | Spindle Direction |
| **Pin 13** | `SpnDir` | LED / Spindle Dir |
| **Pin 4** | `Z.STEP` | Krok osy Z |
| **Pin 7** | `Z.DIR` | Směr osy Z |
| **Pin A3** | `CoolEn` | Chlazení (Analog 3) |

- **Úhly serva pera:**
  - `P1` = **`180°`** (Pero NAHOŘE / zdvih ze souřadnicové plochy)
  - `P0` = **`90°`** (Pero DOLE / kreslení přítlakem na papír)
  - `P<5..180>` = libovolný přímý úhel ve stupních pro jemné ladění přítlaku.

---

## 3. Kinematika & Kalibrované konstanty

Kinematika převádí kartézské souřadnice $(X, Y)$ na změnu délky dvou lanek $(L_A, L_B)$.

```
   Motor A (-370, 500)                      Motor B (+370, 500)
         \                                      /
          \                                    /
           \                                  /
            \---[L_A]                  [L_B]-/
                     \                /
                      +---[Gondola]---+
                      |   (45 mm)     |
                      |     (.)       |  <- Pero v počátku (0, 0)
                      +---------------+
```

### Kalibrované hodnoty (`main.go` & `main.c`)
- **Převodní poměr kroků:**
  $$\text{StepsPerMmLeft} = \text{StepsPerMmRight} = \frac{10000\text{ kroků}}{52.5\text{ mm}} = 190.476190476\text{ steps/mm}$$
- **Souřadnice kotevních motorů:**
  - $\text{MotorAX} = -370.0\text{ mm}$, $\text{MotorAY} = 500.0\text{ mm}$
  - $\text{MotorBX} = +370.0\text{ mm}$, $\text{MotorBY} = 500.0\text{ mm}$
  - Rozteč kotev motorů: **`740.0 mm`** (případně nastavitelná parametrem `-w` od `525.0 mm`).
- **Šířka gondoly (`gondolaWidth`):** **`45.0 mm`** (úchyty šňůr jsou posunuty o $\pm 22.5\text{ mm}$ od osy pera).
- **Výchozí délky šňůr v nulovém bodě `(0, 0)`:**
  $$L_{A0} = \sqrt{(0 - \text{MotorAX} - 22.5)^2 + (0 - \text{MotorAY})^2} = \sqrt{347.5^2 + 500^2} \approx 608.88\text{ mm}$$
  $$L_{B0} = \sqrt{(0 - \text{MotorBX} + 22.5)^2 + (0 - \text{MotorBY})^2} = \sqrt{(-347.5)^2 + 500^2} \approx 608.88\text{ mm}$$

### Segmentace trajektorií
Aby se křivky a přímky nedeformovaly v nelineárním poli V-plotteru, rozřezává hostitel dráhu na malé lineární segmenty:
- **Kreslicí segment (`segmentSize`):** `4.0 mm` (min limit `0.01 mm`).
- **Přejezdový segment:** `8.0 mm`.

---

## 4. Sériový komunikační protokol (`115200` baud)

Komunikace probíhá textovými ASCII řádky zakončenými znakem nového řádku (`\n` nebo `\r\n`). Po dokončení každého příkazu odpovídá Arduino řetězcem `ok\n`.

| Příkaz | Argumenty | Funkce | Příklad |
|:---|:---|:---|:---|
| `X...Y...` | `X<long>Y<long>` | Absolutní krokový posun obou motorů od počátku | `X1200Y-850\n` |
| `P1` | — | Zvednutí pera (úhel `180°`) | `P1\n` |
| `P0` | — | Spuštění pera (úhel `90°`) | `P0\n` |
| `P...` | `P<5..180>` | Nastavení vlastního úhlu serva | `P120\n` |
| `S...` | `S<us>` | Zpoždění kroku v mikrosekundách (rychlost) | `S300\n` |
| `F...` | `F<float>` | Násobič feedrate | `F1.2\n` |
| `E1` / `E0` | — | Zapnutí (`E1`) / vypnutí (`E0`) buzení motorů | `E1\n` |
| `Z0` | — | Vynulování čítačů kroků v aktuální pozici | `Z0\n` |

### Protokol nouzového zastavení (E-Stop)
Při stisku `Ctrl+C` nebo zavolání `drkota estop`:
1. Hostitel okamžitě odešle `P1` (okamžité zvednutí pera z papíru).
2. Hostitel odešle `Z0` (vynulování počátku v místě zastavení).
3. **Důsledek:** Zabrání se stržení šňůr nebo návratu do starého bodu přes nakreslený výkres.

---

## 5. Nástroje flotily & CLI reference

Všechny nástroje jsou přístupné v systémové cestě `~/bin/`:

### Základní CLI (`~/bin/drkota`)
```bash
drkota status               # Kontrola spojení a přítomnosti /dev/ttyUSB0
drkota pen up               # Zvednutí pera (P1)
drkota pen down             # Spuštění pera (P0)
drkota raw P130             # Testovací úhel serva (5..180)
drkota zero                 # Vynulování (0,0) na aktuální pozici
drkota move 50 50 1         # Pohyb na X=50, Y=50 se spuštěným perem
drkota move 0 0 0           # Přejezd zpět do počátku se zvednutým perem
drkota estop                # Bezpečný E-Stop
drkota plot soubor.txt      # Vykreslení seznamu bodů (X Y PEN)
drkota text "POZDRAV"       # Kreslení jednořádkového textu
```

### Hlavní SVG Plotter (`~/src/gomotor/gomotor`)
```bash
gomotor --file kresba.svg --center --fit       # Vycentrovat a přizpůsobit formátu A4
gomotor --file kresba.svg --speed 250 --feed 1.2
```
- **Hlavní přepínače:**
  - `--file <cesta>`: Vstupní SVG soubor.
  - `--port <dev>`: Sériový port (výchozí `/dev/ttyUSB0`).
  - `--center`: Vycentrování na souřadnici `(0, 0)`.
  - `--fit`: Přizpůsobení velikosti A4 (`210 × 297 mm` s `10 mm` okrajem).
  - `--minlift <mm>`: Přeskočení zvedání pera pro krátké přejezdy (< `2.0 mm`).
  - `--inverty`: Zrcadlení vertikální osy Y (výchozí `true`).

### Sazba a tisk článků A4 (`~/src/gomotor/plot_clanek.py`)
- **Písmo:** Vektorový Hershey/Bézier font s kompletní českou diakritikou (`ěščřžýáíéůúďťň`).
- **Rozměry:** Výška verzálek `3.2 mm`, střední výška `2.2 mm`, řádkování `5.4 mm`.
- **Boustrofédon:** Sudé řádky zleva doprava, liché zprava doleva (tam a zpět).
```bash
python3 ~/src/gomotor/plot_clanek.py ~/src/konzdat/clanky/17_jak_bych_to_jeste_jen_nevzpomnel/clanek.md clanek17.svg 20
drkota plot clanek17.svg
```

### Mikro-texty a palindromatické křivky (`~/src/gomotor/gen_tiny.py`)
- **Rozměry:** Výška verzálek `2.5 mm`, šířka textového bloku `23.45 × 16.80 mm`.
- **Reverzace oblouků:** Zrcadlení kontrolních bodů kubických Bézierů ($C_1 \leftrightarrow C_2, P_0 \leftrightarrow P_1$) bez deformace smyček.
```bash
python3 ~/src/gomotor/gen_tiny.py tiny_tiny.svg
drkota plot tiny_tiny.svg
```

### Lokální AI asistent (`~/bin/drkota-qwen`)
```bash
drkota-qwen "nakresli čtverec 40x40 mm a vycentruj ho"     # Dry-run
drkota-qwen -x "zvedni pero a sjeď o 30 mm níž"           # Ostré vykonání
```

### Flashování firmwaru (`~/bin/drkota-flash`)
```bash
drkota-flash uno           # Arduino Uno (ATmega328P, 115200 baud)
drkota-flash nano-old      # Arduino Nano se starým bootloaderem (57600 baud)
```

---

## 6. Bezpečnostní a údržbový protokol

1. **Před zapnutím napájení:** Ručně ověř, že se šňůry z navijáků odvíjejí hladce a nikde se nekříží.
2. **Nulování gondoly:** Po zavěšení gondoly s perem do středu papíru vždy spusť `drkota zero`.
3. **Konec kreslení:** Vždy proveď zvednutí pera (`drkota pen up`), aby hrot nevpíjel inkoust do papíru.
4. **Výměna média:** Při výměně papíru A4 doporučujeme použít papírovou malířskou pásku v rozích.

--τέχνη & kof
