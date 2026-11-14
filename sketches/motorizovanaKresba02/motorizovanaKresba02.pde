import processing.serial.*;

Serial myPort;
Moto myMoto;

void setup() {
  size(800, 600);
  
  // Inicializace sériového portu (uprav index v Serial.list()[0] dle potřeby)
  printArray(Serial.list());
  String portName = Serial.list()[0]; 
  myPort = new Serial(this, portName, 115200);
  myPort.write("F" + 100.0 + "\n");
  
  myMoto = new Moto(width/2, height/2);
}

void draw() {
  background(30);
  
  // Ovládání: cíl (target) je pozice myši
  // myMoto se k němu přibližuje fyzikálním modelem
  myMoto.update(mouseX, mouseY);
  myMoto.display();
  
  // Posílání příkazů do Arduina (jen když se pohne)
  // Omezujeme na cca 30 Hz, aby se nezahltil serial buffer
  if (frameCount % 2 == 0) {
    sendToArduino(myMoto.pos);
  }
}

void sendToArduino(PVector p) {
  // Převod na formát X...Y...
  // Předpokládám, že Arduino očekává souřadnice 0-800, 0-600
  // Prohození os:
  float x = p.y;
  float y = p.x;
  // Příklad: prohození os a obrácení X
  int finalX = (int)(p.x); 
  int finalY = (int)(height - p.y);

  String cmd = "X" + (int)finalX + "Y" + (int)finalY + "\n";
  myPort.write(cmd);

  //float speed = map(myMoto.getSpeedFactor(), 0.01, 10, 0.2, 10.0);
  //myPort.write("F" + speed + "\n");

}

// --- TŘÍDA MOTO ---
class Moto {
  PVector pos;
  PVector vel;
  float friction = 0.85; // Odpor prostředí
  float spring = 0.15;   // Síla tahu za myší
  
  Moto(float x, float y) {
    pos = new PVector(x, y);
    vel = new PVector(0, 0);
  }
  
  void update(float targetX, float targetY) {
    PVector target = new PVector(targetX, targetY);
    PVector force = PVector.sub(target, pos);
    
    force.mult(spring);
    vel.add(force);
    vel.mult(friction);
    pos.add(vel);
  }

  float getSpeedFactor() {
  // Vrátí velikost vektoru rychlosti (0.0 až cca 10.0)
  return vel.mag();
  }
  
  void display() {
    stroke(0, 255, 100);
    strokeWeight(4);
    noFill();
    ellipse(pos.x, pos.y, 40, 40);
    line(pos.x - 10, pos.y, pos.x + 10, pos.y);
    line(pos.x, pos.y - 10, pos.x, pos.y + 10);
  }
}

// Reakce na tlačítko myši - pero dolů/nahoru
void mousePressed() {
  myPort.write("P1\n"); // Pero dolů
}

void mouseReleased() {
  myPort.write("P0\n"); // Pero nahoru
}
