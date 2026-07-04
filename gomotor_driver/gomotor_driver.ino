const byte X_STEP_PIN = 2;
const byte X_DIR_PIN  = 5;
const byte Y_STEP_PIN = 3;
const byte Y_DIR_PIN  = 6;
const byte ENABLE_PIN = 8;
const byte PEN_PIN    = 9;

const bool INVERT_X = false; 
const bool INVERT_Y = false; 

// Parametry akcelerace (nyní proměnné)
unsigned int cruiseDelay = 300; // Základní rychlost (nižší = rychlejší)
const unsigned int START_DELAY = 1000;
const int RAMP_STEPS = 10;

long currentX = 0;
long currentY = 0;
float feedMult = 1.0; // Násobitel pro detailní práci

char serialBuffer[32];
int bufIndex = 0;

void setup() {
    pinMode(X_STEP_PIN, OUTPUT);
    pinMode(X_DIR_PIN, OUTPUT);
    pinMode(Y_STEP_PIN, OUTPUT);
    pinMode(Y_DIR_PIN, OUTPUT);
    pinMode(ENABLE_PIN, OUTPUT);
    pinMode(PEN_PIN, OUTPUT);
    
    digitalWrite(ENABLE_PIN, LOW);
    digitalWrite(PEN_PIN, LOW);
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

    long actualRamp = min((long)RAMP_STEPS, maxSteps / 2);
    unsigned int currentDelay = START_DELAY;
    long rampDelta = (long)(START_DELAY - cruiseDelay);
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

        delayMicroseconds(2);

        if (doStepX) digitalWrite(X_STEP_PIN, LOW);
        if (doStepY) digitalWrite(Y_STEP_PIN, LOW);

        if (actualRamp > 0) {
            if (i < actualRamp) {
                accError += rampDelta;
                while (accError >= actualRamp) {
                    currentDelay--;
                    accError -= actualRamp;
                }
            } else if (i >= maxSteps - actualRamp) {
                if (i == maxSteps - actualRamp) accError = 0;
                accError += rampDelta;
                while (accError >= actualRamp) {
                    currentDelay++;
                    accError -= actualRamp;
                }
            } else {
                currentDelay = cruiseDelay;
            }
        } else {
            currentDelay = cruiseDelay;
        }

        if (currentDelay < cruiseDelay) currentDelay = cruiseDelay;
        if (currentDelay > START_DELAY) currentDelay = START_DELAY;

        // Aplikace zpomalení
        delayMicroseconds((unsigned int)(currentDelay / feedMult));
    }
}

void loop() {
    while (Serial.available() > 0) {
        char c = Serial.read();
        
        if (c == '\n' || c == '\r') {
            if (bufIndex > 0) {
                // Příkazy:
                // X...Y... -> Pohyb
                // P0/P1    -> Pero
                // F0.5     -> Násobitel rychlosti (Feedrate)
                // S200     -> Nastavení základní rychlosti (Cruise Delay)
                
                if (serialBuffer[0] == 'X') {
                    long targetX = 0, targetY = 0;
                    if (sscanf(serialBuffer, "X%ldY%ld", &targetX, &targetY) == 2) {
                        stepPlotter(targetX, targetY);
                    }
                } 
                else if (serialBuffer[0] == 'P') {
                    int state = atoi(&serialBuffer[1]);
                    digitalWrite(PEN_PIN, state ? HIGH : LOW);
                } 
                else if (serialBuffer[0] == 'F') {
                    feedMult = atof(&serialBuffer[1]);
                }
                else if (serialBuffer[0] == 'S') {
                    cruiseDelay = atoi(&serialBuffer[1]);
                }
                
                Serial.print("ok\n"); 
                bufIndex = 0;
            }
        } 
        else if (bufIndex < 31) {
            serialBuffer[bufIndex++] = c;
            serialBuffer[bufIndex] = '\0';
        }
    }
}