// Configureerbare identifier voor deze Arduino
const char* IDENTIFIER = "CLIPMANAGER_TEST"; // Verander dit naar bijv. "CLIPMANAGER_HYPE" of "CLIPMANAGER_BLUNDER"

int buttonState = 1; // INPUT_PULLUP reverses logic, so 1 is actually 0 (high == low); default buttonstate = low
const int buttonPin = 12;
int previousState = 1; // Initial 'previous state' is low

void setup() {
  pinMode(buttonPin, INPUT_PULLUP); 
  Serial.begin(9600);
  // Stuur een bericht bij opstarten om te bevestigen dat de Arduino werkt
  Serial.println("Arduino gestart met identifier: " + String(IDENTIFIER));
}

void loop() {
  // Check for identification request from the PC
  if (Serial.available() > 0) {
    String data = Serial.readString();
    data.trim();
    if (data == "IDENTIFY") {
      Serial.println(IDENTIFIER); // Stuur de identifier naar de PC
    }
  }

  buttonState = digitalRead(buttonPin);

  // If the button state changes
  if (buttonState != previousState) {
    // Remember, 1 is low (off), 0 is high (on)
    if (buttonState == 0) { // Button is pressed (LOW due to INPUT_PULLUP)
      Serial.println("BUTTON_PRESSED");
    }
    previousState = buttonState;
  }
  delay(50); // Debouncing
}