// MultiStepper.pde
// -*- mode: C++ -*-
//
// Shows how to multiple simultaneous steppers
// Runs one stepper forwards and backwards, accelerating and decelerating
// at the limits. Runs other steppers at the same time
//
// Copyright (C) 2009 Mike McCauley
// $Id: MultiStepper.pde,v 1.1 2011/01/05 01:51:01 mikem Exp mikem $

#include <AccelStepper.h>





AccelStepper Xaxis(1, 2, 5); // pin 2 = step, pin 5 = direction
AccelStepper Yaxis(1, 3, 6); // pin 2 = step, pin 5 = direction

const byte enablePin = 8;

void setup()
{
   pinMode(enablePin, OUTPUT);
   digitalWrite(enablePin, LOW);

   Xaxis.setMaxSpeed(12800);
   Xaxis.setAcceleration(100.0);
   Xaxis.setSpeed(1000); // had to slow for my motor

Yaxis.setMaxSpeed(12800);
   Yaxis.setAcceleration(100.0);
   Yaxis.setSpeed(1000); // had to slow for my motor


   Xaxis.moveTo(2400);
}

void loop()
{
    // Change direction at the limits
    if (Xaxis.distanceToGo() == 0)
	Xaxis.moveTo(-Xaxis.currentPosition());
    Xaxis.run();
    
    // Change direction at the limits
    if (Yaxis.distanceToGo() == 0)
	Yaxis.moveTo(-Yaxis.currentPosition());
    Yaxis.run();
    

}

