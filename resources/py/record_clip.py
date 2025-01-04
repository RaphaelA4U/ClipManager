import os
from datetime import datetime

def record_clip():
    now = datetime.now().strftime("%Y-%m-%d_%H-%M-%S")
    os.system(f"ffmpeg -i rtsp://<camera_ip>:554/h264Preview_01_main -t 30 -vcodec copy /var/www/html/clips/{now}.mp4")

record_clip()
