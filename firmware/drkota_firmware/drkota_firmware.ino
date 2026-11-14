#include <Servo.h>

Servo penServo;

const byte X_STEP_PIN = 2;
const byte X_DIR_PIN  = 5;
const byte Y_STEP_PIN = 3;
const byte Y_DIR_PIN  = 6;
const byte ENABLE_PIN = 8;
const byte PEN_PIN    = 11; // Z+ header on CNC shield

const bool INVERT_X = false; 
const bool INVERT_Y = false; 

// Servo úhly (1 = zvedá pero od plochy, 0 = pokládá na plochu)
const int PEN_UP_ANGLE   = 180; // 1 zvedá pero od plochy
const int PEN_DOWN_ANGLE = 90;  // 0 pokládá pero na plochu

// Parametry akcelerace
unsigned int cruiseDelay = 300; 
const unsigned int START_DELAY = 1000;
const int RAMP_STEPS = 50;

long currentX = 0;
long currentY = 0;
float feedMult = 1.0; 

char serialBuffer[32];
int bufIndex = 0;

void setup() {
    pinMode(X_STEP_PIN, OUTPUT);
    pinMode(X_DIR_PIN, OUTPUT);
    pinMode(Y_STEP_PIN, OUTPUT);
    pinMode(Y_DIR_PIN, OUTPUT);
    pinMode(ENABLE_PIN, OUTPUT);
    
    // Aktivace stepper driverů (Enable je aktivní v LOW)
    digitalWrite(ENABLE_PIN, LOW);
    
    // Inicializace serva - startujeme v poloze ZVEDNUTO od plochy (bezpečnost)
    penServo.attach(PEN_PIN);
    penServo.write(PEN_UP_ANGLE); 
    
    Serial.begin(115200);
    Serial.println("DRKOTA READY");
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

    unsigned int currentDelay = (unsigned int)(cruiseDelay / feedMult);
    if (currentDelay < 50) currentDelay = 50;

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

        delayMicroseconds(currentDelay);
    }
}

void loop() {
    while (Serial.available() > 0) {
        char c = Serial.read();
        
        if (c == '\n' || c == '\r') {
            if (bufIndex > 0) {
                // Příkazy:
                // X<x>Y<y> -> absolutní pohyb v krocích od nuly
                // P1       -> zvednout pero od plochy (PEN_UP_ANGLE = 180)
                // P0       -> položit pero na plochu (PEN_DOWN_ANGLE = 90)
                // P<úhel>  -> přímé nastavení úhlu serva (např. P140) pro ladění
                // F<mult>  -> násobitel rychlosti
                // S<delay> -> základní zpoždění (nižší = rychlejší)
                // E0 / E1  -> Enable / Disable stepper motorů
                // Z0       -> Vynulování aktuální souřadnice (Current position = 0, 0)
                
                if (serialBuffer[0] == 'X') {
                    long targetX = 0, targetY = 0;
                    if (sscanf(serialBuffer, "X%ldY%ld", &targetX, &targetY) == 2) {
                        stepPlotter(targetX, targetY);
                    }
                } 
                else if (serialBuffer[0] == 'P') {
                    int val = atoi(&serialBuffer[1]);
                    if (val == 1) {
                        penServo.write(PEN_UP_ANGLE); // 1 = zvedá pero od plochy
                    } else if (val == 0) {
                        penServo.write(PEN_DOWN_ANGLE); // 0 = pokládá pero na plochu
                    } else if (val >= 10 && val <= 180) {
                        penServo.write(val); // přímý úhel pro ladění
                    }
                } 
                else if (serialBuffer[0] == 'F') {
                    feedMult = atof(&serialBuffer[1]);
                    if (feedMult <= 0.01) feedMult = 1.0;
                } 
                else if (serialBuffer[0] == 'S') {
                    cruiseDelay = atoi(&serialBuffer[1]);
                    if (cruiseDelay < 50) cruiseDelay = 50;
                }
                else if (serialBuffer[0] == 'E') {
                    int en = atoi(&serialBuffer[1]);
                    digitalWrite(ENABLE_PIN, en ? LOW : HIGH); // LOW = Enabled
                }
                else if (serialBuffer[0] == 'Z') {
                    currentX = 0;
                    currentY = 0;
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
