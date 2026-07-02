#include <AccelStepper.h>

// =========================================================================
// CHYTRÁ KALIBRACE MIKROKROKOVÁNÍ
// Sem zadej PŘESNĚ STEJNÉ číslo, jaké máš v Go kódu v proměnné StepsPerMm!
// Pokud máš základní krok, je tu 200.0. Pokud máš mikrokroky, dej sem např. 1600.0 nebo 3200.0
const float STEPS_PER_MM = 200.0; 
// =========================================================================

AccelStepper Xaxis(1, 2, 5); 
AccelStepper Yaxis(1, 3, 6); 

const byte enablePin = 8;
char serialBuffer[32];
int bufIndex = 0;
bool isMoving = false;

void setup() {
    pinMode(enablePin, OUTPUT);
    digitalWrite(enablePin, LOW); // LOW povolí motory na CNC Shieldu

    Serial.begin(115200);

    // OPRAVA ČÍSLO 1: AKCELERACE A RYCHLOST
    // Protože Go kód posílá čáry rozkouskované po 1 mm a čeká na "ok", nízká akcelerace
    // způsobovala, že motor na každém milimetru zdlouhavě zrychloval a brzdil.
    // To vedlo k šílenému škubání a deformaci obrazu. Nastavujeme "okamžitou" reakci.
    Xaxis.setMaxSpeed(30000);
    Xaxis.setAcceleration(999999.0); // Téměř nekonečná akcelerace, krokují hned
    Yaxis.setMaxSpeed(30000);
    Yaxis.setAcceleration(999999.0);

    // 💡 TIP FOR INVERSION: Pokud se ti po spuštění některý motor točí obráceně,
    // odkomentuj jeden z následujících řádků (otočí směr softwarově):
    // Xaxis.setPinsInverted(true, false, false);
    // Yaxis.setPinsInverted(true, false, false);

    // OPRAVA ČÍSLO 2: DYNAMICKÝ START (Proč mašina utíkala)
    // Původní číslo 117796 natvrdo počítalo s 200 kroky na mm. Když jsi v Go kódu 
    // změnil měřítko/mikrokroky, Go poslalo startovní pozici např. 942000, ale Arduino 
    // si myslelo, že stojí na 117796. Rozdíl byl obří a mašina okamžitě vystřelila do rohu.
    // Teď se startovní pozice dopočítá sama podle zadaných STEPS_PER_MM (vzdálenost ke středu je 588.98 mm).
    long startSteps = 565.5 * STEPS_PER_MM;
    Xaxis.setCurrentPosition(startSteps); 
    Yaxis.setCurrentPosition(startSteps); 
}

void loop() {
    Xaxis.run();
    Yaxis.run();

    while (Serial.available() > 0) {
        char c = Serial.read();
        
        if (c == '\n' || c == '\r') {
            if (bufIndex > 0) {
                long targetX = 0, targetY = 0;
                long maxSpd = 0, spd = 0, accel = 0;

                // Parsování pohybu
                if (serialBuffer[0] == 'X' && sscanf(serialBuffer, "X%ldY%ld", &targetX, &targetY) == 2) {
                    Xaxis.moveTo(targetX);
                    Yaxis.moveTo(targetY);
                    isMoving = true;
                }
                // Parsování konfigurace X
                else if (serialBuffer[0] == 'Q' && sscanf(serialBuffer, "Q%ldW%ldE%ld", &maxSpd, &spd, &accel) == 3) {
                    Xaxis.setMaxSpeed(maxSpd);
                    Xaxis.setAcceleration(accel);
                    Serial.print("ok\n");
                }
                // Parsování konfigurace Y
                else if (serialBuffer[0] == 'A' && sscanf(serialBuffer, "A%ldS%ldD%ld", &maxSpd, &spd, &accel) == 3) {
                    Yaxis.setMaxSpeed(maxSpd);
                    Yaxis.setAcceleration(accel);
                    Serial.print("ok\n");
                }
                bufIndex = 0; 
            }
        } 
        else if (bufIndex < 31) {
            serialBuffer[bufIndex++] = c;
            serialBuffer[bufIndex] = '\0';
        }
    }

    // Kontrola dojezdu
    if (isMoving && Xaxis.distanceToGo() == 0 && Yaxis.distanceToGo() == 0) {
        Serial.print("ok\n");
        isMoving = false;
    }
}