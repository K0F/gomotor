import processing.svg.*;

void setup() {
  size(576, 814);
  noiseSeed(26);
  beginRecord(SVG, "01_plotter.svg");

  stroke(0);
  noFill();

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

  endRecord();
  println("Hotovo.");
  exit();
}
