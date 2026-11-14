#include <Servo.h>

Servo penServo;

const byte X_STEP_PIN = 2; // PORTD2
const byte X_DIR_PIN  = 5; // PORTD5
const byte Y_STEP_PIN = 3; // PORTD3
const byte Y_DIR_PIN  = 6; // PORTD6
const byte ENABLE_PIN = 8; // PORTB0
const byte PEN_PIN    = 11; // Z+ header on CNC shield

const bool INVERT_X = false;
const bool INVERT_Y = false;

// Servo angles (1 = lifts pen from surface, 0 = lowers pen to surface)
const int PEN_UP_ANGLE   = 180; // 1 = pen UP
const int PEN_DOWN_ANGLE = 90;  // 0 = pen DOWN
bool holdServo = false;         // Keep PWM active if needed (H1 / H0)
int currentPenAngle = PEN_UP_ANGLE;

// Timing and smoothing parameters
unsigned int cruiseDelay = 300;
const unsigned int START_DELAY = 900;
const long RAMP_THRESHOLD = 400; // Apply smooth ramp only on moves > 400 steps (~2.1 mm)
const long RAMP_STEPS     = 80;

long currentX = 0;
long currentY = 0;
int lastSignX = 0;
int lastSignY = 0;
float feedMult = 1.0;

char serialBuffer[32];
int bufIndex = 0;

void movePenSmooth(int angle) {
    currentPenAngle = angle;
    if (!penServo.attached()) {
        penServo.attach(PEN_PIN);
    }
    penServo.write(angle);
    delay(250);
    // Detach Timer1 ISR while stepping so stepper pulse timing is 100% deterministic (zero jitter)
    if (!holdServo) {
        penServo.detach();
        digitalWrite(PEN_PIN, LOW);
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

    // Initialize servo in UP position and detach Timer1 ISR
    movePenSmooth(PEN_UP_ANGLE);

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
    // Direction setup guard time for A4988 / DRV8825
    delayMicroseconds(5);

    long maxSteps = max(stepsX, stepsY);
    long overX = maxSteps / 2;
    long overY = maxSteps / 2;

    unsigned int targetDelay = (unsigned int)(cruiseDelay / feedMult);
    if (targetDelay < 60) targetDelay = 60;

    // Only ramp if this is a long move or a sharp reversal; short collinear segments run at continuous cruise speed
    bool dirChanged = (stepsX > 20 && signX != lastSignX && lastSignX != 0) ||
                      (stepsY > 20 && signY != lastSignY && lastSignY != 0);
    bool useRamp = (maxSteps >= RAMP_THRESHOLD) || dirChanged;
    long actualRamp = useRamp ? min(RAMP_STEPS, maxSteps / 3) : 0;
    unsigned int startD = max(targetDelay, START_DELAY);

    for (long i = 0; i < maxSteps; i++) {
        byte stepMask = 0;

        overX += stepsX;
        if (overX >= maxSteps) {
            overX -= maxSteps;
            stepMask |= (1 << X_STEP_PIN);
            currentX += signX;
        }

        overY += stepsY;
        if (overY >= maxSteps) {
            overY -= maxSteps;
            stepMask |= (1 << Y_STEP_PIN);
            currentY += signY;
        }

        // Simultaneous atomic step pulse on PORTD (pins 2 & 3)
        PORTD |= stepMask;
        delayMicroseconds(4);
        PORTD &= ~stepMask;

        unsigned int d = targetDelay;
        if (actualRamp > 0) {
            if (i < actualRamp) {
                d = startD - (unsigned int)(((long)(startD - targetDelay) * i) / actualRamp);
            } else if (i >= maxSteps - actualRamp) {
                long rem = maxSteps - 1 - i;
                d = startD - (unsigned int)(((long)(startD - targetDelay) * rem) / actualRamp);
            }
        }
        delayMicroseconds(d);
    }

    if (stepsX > 0) lastSignX = signX;
    if (stepsY > 0) lastSignY = signY;
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
                        movePenSmooth(PEN_UP_ANGLE);
                    } else if (val == 0) {
                        movePenSmooth(PEN_DOWN_ANGLE);
                    } else if (val >= 10 && val <= 180) {
                        movePenSmooth(val);
                    }
                }
                else if (serialBuffer[0] == 'H') {
                    holdServo = (atoi(&serialBuffer[1]) != 0);
                    if (holdServo) {
                        penServo.attach(PEN_PIN);
                        penServo.write(currentPenAngle);
                    } else {
                        penServo.detach();
                        digitalWrite(PEN_PIN, LOW);
                    }
                }
                else if (serialBuffer[0] == 'F') {
                    feedMult = atof(&serialBuffer[1]);
                    if (feedMult <= 0.01) feedMult = 1.0;
                }
                else if (serialBuffer[0] == 'S') {
                    cruiseDelay = atoi(&serialBuffer[1]);
                    if (cruiseDelay < 60) cruiseDelay = 60;
                }
                else if (serialBuffer[0] == 'E') {
                    int en = atoi(&serialBuffer[1]);
                    digitalWrite(ENABLE_PIN, en ? LOW : HIGH);
                }
                else if (serialBuffer[0] == 'Z') {
                    currentX = 0;
                    currentY = 0;
                    lastSignX = 0;
                    lastSignY = 0;
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
