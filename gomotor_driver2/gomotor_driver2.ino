const byte X_STEP_PIN = 2;
const byte X_DIR_PIN  = 5;
const byte Y_STEP_PIN = 3;
const byte Y_DIR_PIN  = 6;
const byte ENABLE_PIN = 8;
const byte PEN_PIN    = 9;

const bool INVERT_X = false; 
const bool INVERT_Y = false; 

…        } 
        else if (bufIndex < 31) {
            serialBuffer[bufIndex++] = c;
            serialBuffer[bufIndex] = '\0';
        }
    }
}