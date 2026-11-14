import processing.svg.*;


boolean render = false;

void setup() {
  size(576, 814);
  noiseSeed(26);
  smooth();
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
  strokeWeight(3);
  noFill();


  pushMatrix();

  int rmax = 180;
  int segments = 360;
  int rstep = 8;
  
  translate(width/2-rmax/2.0,height/5*2);

  for(float r = rmax; r > rstep; r -= rstep){
      beginShape();
  
    for(float i = 0; i < segments; i++) {
      float angle = map(i, 0, segments, 0, TWO_PI);
      float x = r * cos(angle);
      float y = r * sin(angle);
      vertex(x, y);
    }
      endShape();
  }
  popMatrix();

  pushMatrix();
  translate(width/2+rmax/2.0,height/5*2);

  for(float r = rmax; r > rstep; r -= rstep){
      beginShape();
  
    for(float i = 0; i < segments; i++) {
      float angle = map(i, 0, segments, 0, TWO_PI);
      float x = r * cos(angle);
      float y = r * sin(angle);
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
