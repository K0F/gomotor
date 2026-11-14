import processing.svg.*;

void setup() {
  size(576, 814, SVG, "01.svg");
}

void draw() {
  float step = 10.0;
  // Draw something good here
  
  stroke(0);
  noFill();
  for(float i = 0 ; i < height;i+=step){
  beginShape();
    for(float ii = 1 ; ii < width;ii++){
      float sinus = sin((ii*TWO_PI)/150.0)*step;
      vertex(ii, i+sinus);
    }
  endShape();
  
  }

  // Exit the program
  println("Finished.");
  exit();
}
