import processing.svg.*;


boolean render = false;

void setup() {
  size(576, 814, P3D);
  noiseSeed(26);
  sphereDetail(14);
//  ortho();
}

void keyPressed(){
  if(key == ' ')
    render = true;
}

void draw(){
  background(255);

  if(render)
    beginRecord(SVG, "drawing.svg");
  
  stroke(0);
  noFill();


  pushMatrix();
  translate(width/2,height/2,0);
  

  float r = 200;
  ellipse(0, 0, r*2, r*2);

  int lines = 20;
  for(int i = 1; i < lines; i++) {
    float angle = map(i, 0, lines, -HALF_PI, HALF_PI);
    float w = r * cos(angle);
    float h = r * sin(angle);
    ellipse(0, h, w*2, w*2 * 0.3);
  }

  for(int i = 0; i < 20; i++) {
    float angle = map(i, 0, 20, 0, TWO_PI);
    beginShape();
    for(float a = -HALF_PI; a <= HALF_PI; a += 0.1) {
      float x = r * cos(a) * cos(angle);
      float y = r * sin(a);
      vertex(x, y);
    }
    endShape();
  }

  popMatrix();


  if(render){
    endRecord();
    println("Hotovo.");
    exit();
  }
}
 void vlny(){

  float step = 7.0;
  boolean leftToRight = true;



  beginShape();
  for (float y = 0; y < height; y += step) {
    if (leftToRight) {
      for (float x = 0; x <= width; x += step) {
        float sinus = (noise(y / 100.0, x / 100.0) - 0.5) * 100.0;
        vertex(x, y + sinus);
      }
    } else {
      for (float x = width; x >= 0; x -= 5) {
        float sinus = (noise(y / 100.0, x / 100.0) -0.5 )* 100.0;
        vertex(x, y + sinus);
      }
    }
    leftToRight = !leftToRight;
  }
  endShape();

 }
