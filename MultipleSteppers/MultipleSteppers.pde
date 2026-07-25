#include <AccelStepper.h>

// Definice motorů (krokový pin, směrový pin)
AccelStepper Xaxis(1, 2, 5); // pin 2 = step, pin 5 = direction
AccelStepper Yaxis(1, 3, 6); // pin 3 = step, pin 6 = direction

const byte enablePin = 8;

void setup() {
  // Komunikace s Go programem
  Serial.begin(115200);

  pinMode(enablePin, OUTPUT);
  digitalWrite(enablePin, LOW); // Zapnutí driverů

  Xaxis.setMaxSpeed(8000);
  Xaxis.setAcceleration(4000);
  
  Yaxis.setMaxSpeed(8000);
  Yaxis.setAcceleration(4000);
}

void loop() {
  // Čtení příkazů ze sériového portu
  if (Serial.available() > 0) {
    String line = Serial.readStringUntil('\n');
    line.trim();
    if (line.length() == 0) return;

    // Zpracování příkazu pro tužku (P0 / P1)
    if (line.startsWith("P")) {
      // Zde případně přidejte kód pro ovládání serva/relé tužky na pinu
      Serial.println("ok");
    }
    // Zpracování nastavení rychlosti (S...)
    else if (line.startsWith("S")) {
      float spd = line.substring(1).toFloat();
      if (spd > 0) {
        Xaxis.setMaxSpeed(spd);
        Yaxis.setMaxSpeed(spd);
      }
      Serial.println("ok");
    }
    // Zpracování rychlostního koeficientu (F...)
    else if (line.startsWith("F")) {
      Serial.println("ok");
    }
    // Zpracování souřadnic pohybu X...Y...
    else {
      int xIndex = line.indexOf('X');
      int yIndex = line.indexOf('Y');

      if (xIndex != -1 && yIndex != -1) {
        long targetX = line.substring(xIndex + 1, yIndex).toInt();
        long targetY = line.substring(yIndex + 1).toInt();

        // Nastavení cíle pro AccelStepper
        Xaxis.moveTo(targetX);
        Yaxis.moveTo(targetY);

        // Provedení pohybu a čekání na dokončení
        while (Xaxis.distanceToGo() != 0 || Yaxis.distanceToGo() != 0) {
          Xaxis.run();
          Yaxis.run();
        }
      }
      // Odeslání potvrzení zpět do Go programu, aby mohl poslat další bod
      Serial.println("ok");
    }
  }
}
