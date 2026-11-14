#include <Servo.h>

// Drive all candidate headers on the CNC Shield V3 simultaneously via Timer1:
// Pin 9  = X+ / X-
// Pin 10 = Y+ / Y-
// Pin 11 = Z+ / Z-
// Pin 12 = SpnEn
// Pin 13 = SpnDir
// Pin 4  = Z_STEP
// Pin 7  = Z_DIR
// Pin A3 = CoolEn
const byte NUM_SERVOS = 8;
const byte SERVO_PINS[NUM_SERVOS] = { 9, 10, 11, 12, 13, 4, 7, A3 };
Servo penServos[NUM_SERVOS];

const byte X_STEP_PIN = 2; // PORTD2
const byte X_DIR_PIN  = 5; // PORTD5
const byte Y_STEP_PIN = 3; // PORTD3
const byte Y_DIR_PIN  = 6; // PORTD6
const byte ENABLE_PIN = 8; // PORTB0

const bool INVERT_X = false;
const bool INVERT_Y = false;

// Servo angles (1 = lifts pen from surface, 0 = lowers pen to surface)
const int PEN_UP_ANGLE   = 180; // 1 = pen UP
const int PEN_DOWN_ANGLE = 90;  // 0 = pen DOWN

unsigned int cruiseDelay = 300;
float feedMult = 1.0;

long currentX = 0;
long currentY = 0;

char serialBuffer[32];
int bufIndex = 0;

void writeAllServos(int angle) {
    for (byte i = 0; i < NUM_SERVOS; i++) {
        penServos[i].write(angle);
    }
}

void setup() {
    pinMode(X_STEP_PIN, OUTPUT);
    pinMode(X_DIR_PIN, OUTPUT);
    pinMode(Y_STEP_PIN, OUTPUT);
    pinMode(Y_DIR_PIN, OUTPUT);
    pinMode(ENABLE_PIN, OUTPUT);

    // Enable stepper drivers (active LOW)
    digitalWrite(ENABLE_PIN, LOW);

    // Attach all candidate CNC Shield headers simultaneously and set initial angle (90)
    for (byte i = 0; i < NUM_SERVOS; i++) {
        penServos[i].attach(SERVO_PINS[i]);
    }
    writeAllServos(90);

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
                if (serialBuffer[0] == 'X') {
                    long targetX = 0, targetY = 0;
                    if (sscanf(serialBuffer, "X%ldY%ld", &targetX, &targetY) == 2) {
                        stepPlotter(targetX, targetY);
                    }
                }
                else if (serialBuffer[0] == 'P') {
                    int val = atoi(&serialBuffer[1]);
                    if (val == 1) {
                        writeAllServos(PEN_UP_ANGLE);   // P1 = 180 (UP)
                    } else if (val == 0) {
                        writeAllServos(PEN_DOWN_ANGLE); // P0 = 90 (DOWN)
                    } else if (val >= 5 && val <= 180) {
                        writeAllServos(val);            // Direct angle on all pins
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
                    digitalWrite(ENABLE_PIN, en ? LOW : HIGH);
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
