// 📌 Definice pinů pro CNC Shield V3
const byte X_STEP_PIN = 2;
const byte X_DIR_PIN = 5;
const byte Y_STEP_PIN = 3;
const byte Y_DIR_PIN = 6;
const byte ENABLE_PIN = 8;

// =========================================================================
// 🎛️ HARDWAROVÁ KONFIGURACE OS
const bool INVERT_X = false; 
const bool INVERT_Y = false; 

// ⏱️ PARAMETRY AKCELERACE (Trapézová rampa)
const unsigned int CRUISE_DELAY = 300; // Prodleva v maximální rychlosti (nižší = rychlejší)
const unsigned int START_DELAY  = 1000; // Bezpečný, pomalý start z klidu (vyšší = pomalejší)
const int RAMP_STEPS = 30;             // Počet kroků pro plný rozjezd a dojezd
// =========================================================================

long currentX = 0;
long currentY = 0;

char serialBuffer[32];
int bufIndex = 0;

void setup() {
    pinMode(X_STEP_PIN, OUTPUT);
    pinMode(X_DIR_PIN, OUTPUT);
    pinMode(Y_STEP_PIN, OUTPUT);
    pinMode(Y_DIR_PIN, OUTPUT);
    pinMode(ENABLE_PIN, OUTPUT);
    
    digitalWrite(ENABLE_PIN, LOW); // Zapnutí motorů
    Serial.begin(115200);
}

void stepPlotter(long targetX, long targetY) {
    long deltaX = targetX - currentX;
    long deltaY = targetY - currentY;

    long stepsX = abs(deltaX);
    long stepsY = abs(deltaY);

    if (stepsX == 0 && stepsY == 0) return;

    int signX = (deltaX > 0) ? 1 : -1;
    int signY = (deltaY > 0) ? 1 : -1;

    digitalWrite(X_DIR_PIN, ((deltaX > 0) ^ INVERT_X) ? HIGH : LOW);
    digitalWrite(Y_DIR_PIN, ((deltaY > 0) ^ INVERT_Y) ? HIGH : LOW);

    long maxSteps = max(stepsX, stepsY);
    long overX = 0;
    long overY = 0;

    // 📐 Optimalizované nastavení rampy
    long actualRamp = min((long)RAMP_STEPS, maxSteps / 2);
    unsigned int currentDelay = START_DELAY;
    
    // Rozdíl rychlostí převedený na long pro akumulátor
    long rampDelta = (long)(START_DELAY - CRUISE_DELAY);
    long accError = 0;

    for (long i = 0; i < maxSteps; i++) {
        overX += stepsX;
        overY += stepsY;

        bool doStepX = false;
        bool doStepY = false;

        if (overX >= maxSteps) {
            overX -= maxSteps;
            digitalWrite(X_STEP_PIN, HIGH);
            doStepX = true;
            currentX += signX;
        }
        if (overY >= maxSteps) {
            overY -= maxSteps;
            digitalWrite(Y_STEP_PIN, HIGH);
            doStepY = true;
            currentY += signY;
        }

        delayMicroseconds(2); // Nutná šířka pulzu pro drivery

        if (doStepX) digitalWrite(X_STEP_PIN, LOW);
        if (doStepY) digitalWrite(Y_STEP_PIN, LOW);

        // =========================================================================
        // ⭐ ULTRA-RYCHLÁ RAMPA BEZ NÁSOBENÍ A DĚLENÍ (Bresenham style)
        // =========================================================================
        if (actualRamp > 0) {
            if (i < actualRamp) {
                // Plynulý rozjezd (snižování delaye pomocí sčítání chybové složky)
                accError += rampDelta;
                while (accError >= actualRamp) {
                    currentDelay--;
                    accError -= actualRamp;
                }
            } 
            else if (i >= maxSteps - actualRamp) {
                // Plynulé brzdění (zvyšování delaye)
                if (i == maxSteps - actualRamp) {
                    accError = 0; // Vynulování akumulátoru přesně na startu brzdné zóny
                }
                accError += rampDelta;
                while (accError >= actualRamp) {
                    currentDelay++;
                    accError -= actualRamp;
                }
            } 
            else {
                currentDelay = CRUISE_DELAY;
            }
        } else {
            currentDelay = CRUISE_DELAY;
        }

        // Bezpečnostní hardwarové limity rychlosti
        if (currentDelay < CRUISE_DELAY) currentDelay = CRUISE_DELAY;
        if (currentDelay > START_DELAY) currentDelay = START_DELAY;

        delayMicroseconds(currentDelay);
    }
}

void loop() {
    while (Serial.available() > 0) {
        char c = Serial.read();
        
        if (c == '\n' || c == '\r') {
            if (bufIndex > 0) {
                long targetX = 0, targetY = 0;

                if (serialBuffer[0] == 'X' && sscanf(serialBuffer, "X%ldY%ld", &targetX, &targetY) == 2) {
                    stepPlotter(targetX, targetY);
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
}