// Configurable identifier for this Arduino
const char* IDENTIFIER = "CLIPMANAGER_TEST"; // Change this to e.g. "CLIPMANAGER_HYPE" or "CLIPMANAGER_BLUNDER"

int buttonState = 1; // INPUT_PULLUP reverses logic, so 1 is actually 0 (high == low); default buttonstate = low
const int buttonPin = 12;
int previousState = 1; // Initial 'previous state' is low
bool startupComplete = false;  // Flag to indicate startup completion

void setup() {
  pinMode(buttonPin, INPUT_PULLUP); 
  Serial.begin(9600);
  // Send a message at startup to confirm the Arduino is working
  Serial.println("Arduino started with identifier: " + String(IDENTIFIER));

  // Startup delay to prevent initial triggers
  delay(5000);  // 5 second delay after startup
  startupComplete = true;
}

void loop() {
  // Check for identification request from the PC
  if (Serial.available() > 0) {
    String data = Serial.readString();
    data.trim();
    if (data == "IDENTIFY") {
      Serial.println(IDENTIFIER); // Send the identifier to the PC
    }
  }

  buttonState = digitalRead(buttonPin);

  // If the button state changes
  if (buttonState != previousState && startupComplete) {
    // Remember, 1 is low (off), 0 is high (on)
    if (buttonState == 0) { // Button is pressed (LOW due to INPUT_PULLUP)
      Serial.println("BUTTON_PRESSED");
    }
    previousState = buttonState;
  }
  delay(50); // Debouncing
}