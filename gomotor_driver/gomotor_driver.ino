#include <Servo.h>

Servo penServo;

const byte X_STEP_PIN = 2; // PORTD2
const byte X_DIR_PIN  = 5; // PORTD5
const byte Y_STEP_PIN = 3; // PORTD3
const byte Y_DIR_PIN  = 6; // PORTD6
const byte ENABLE_PIN = 8; // PORTB0
const byte PEN_PIN_Z  = 11; // Z+ header on CNC shield (Servo)
const byte PEN_PIN_9  = 9;  // X+ header on CNC shield (fallback / digital)

const bool INVERT_X = false;
const bool INVERT_Y = false;

// Servo angles (1 = lifts pen from surface, 0 = lowers pen to surface)
const int PEN_UP_ANGLE   = 180; // 1 = pen UP
const int PEN_DOWN_ANGLE = 90;  // 0 = pen DOWN
byte activePenPin        = PEN_PIN_Z;

unsigned int cruiseDelay = 450; // Safe default for A4988/DRV8825 to prevent thermal/current skipping
float feedMult = 1.0;

long currentX = 0;
long currentY = 0;

char serialBuffer[32];
int bufIndex = 0;

void setPenAngle(int angle) {
    if (!penServo.attached()) {
        penServo.attach(activePenPin);
    }
    penServo.write(angle);
    // Also toggle Pin 9 in case pen is wired to Pin 9
    if (activePenPin != PEN_PIN_9) {
        digitalWrite(PEN_PIN_9, (angle > 120) ? HIGH : LOW);
    }
}

void setup() {
    pinMode(X_STEP_PIN, OUTPUT);
    pinMode(X_DIR_PIN, OUTPUT);
    pinMode(Y_STEP_PIN, OUTPUT);
    pinMode(Y_DIR_PIN, OUTPUT);
    pinMode(ENABLE_PIN, OUTPUT);
    pinMode(PEN_PIN_9, OUTPUT);

    // Enable stepper drivers (active LOW)
    digitalWrite(ENABLE_PIN, LOW);

    // Keep Servo continuously attached on Pin 11 (just like gomotor_driver_w_pen.ino)
    penServo.attach(activePenPin);
    penServo.write(PEN_UP_ANGLE);

    Serial.begin(115200);
    Serial.println("DRKOTA V2 SERVO READY");
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
    delayMicroseconds(10); // Setup time for stepper driver DIR input

    long maxSteps = max(stepsX, stepsY);
    long overX = maxSteps / 2;
    long overY = maxSteps / 2;

    unsigned int currentDelay = (unsigned int)(cruiseDelay / feedMult);
    if (currentDelay < 80) currentDelay = 80;

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

        // 10us pulse width guarantees clean step detection on all A4988/DRV8825 clones
        delayMicroseconds(10);

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
                        setPenAngle(PEN_UP_ANGLE);   // P1 = 180 deg (UP)
                    } else if (val == 0) {
                        setPenAngle(PEN_DOWN_ANGLE); // P0 = 90 deg (DOWN)
                    } else if (val >= 5 && val <= 180) {
                        setPenAngle(val);            // Direct servo angle (e.g. P120)
                    }
                }
                else if (serialBuffer[0] == 'M') {
                    // Switch servo pin at runtime (e.g. M11, M9, M10, M12) to test CNC shield headers
                    int pin = atoi(&serialBuffer[1]);
                    if (pin >= 4 && pin <= 13 && pin != 8) {
                        penServo.detach();
                        activePenPin = (byte)pin;
                        penServo.attach(activePenPin);
                        penServo.write(PEN_UP_ANGLE);
                    }
                }
                else if (serialBuffer[0] == 'F') {
                    feedMult = atof(&serialBuffer[1]);
                    if (feedMult <= 0.01) feedMult = 1.0;
                }
                else if (serialBuffer[0] == 'S') {
                    cruiseDelay = atoi(&serialBuffer[1]);
                    if (cruiseDelay < 80) cruiseDelay = 80;
                }
                else if (serialBuffer[0] == 'E') {
                    int en = atoi(&serialBuffer[1]);
                    digitalWrite(ENABLE_PIN, en ? LOW : HIGH);
                }
                else if (serialBuffer[0] == 'Z') {
                    currentX = 0;
                    currentY = 0;
                }
                else if (serialBuffer[0] == 'V') {
                    Serial.print("DRKOTA_V2_PIN");
                    Serial.print(activePenPin);
                    Serial.print("_DELAY");
                    Serial.print(cruiseDelay);
                    Serial.print(" ");
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
