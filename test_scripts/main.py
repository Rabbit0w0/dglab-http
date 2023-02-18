import time

import requests

POWER = 25
CHANNEL = 1

WAVES = [{"paramX": 1, "paramY": 9, "paramZ": 0}, {"paramX": 1, "paramY": 9, "paramZ": 5},
         {"paramX": 1, "paramY": 9, "paramZ": 0}, {"paramX": 1, "paramY": 9, "paramZ": 10},
         {"paramX": 1, "paramY": 9, "paramZ": 0}, {"paramX": 1, "paramY": 9, "paramZ": 14},
         {"paramX": 1, "paramY": 9, "paramZ": 0}, {"paramX": 1, "paramY": 9, "paramZ": 17},
         {"paramX": 1, "paramY": 9, "paramZ": 0}, {"paramX": 1, "paramY": 9, "paramZ": 20},
         {"paramX": 1, "paramY": 9, "paramZ": 0}]  # 按捏渐强

POWER *= 7  # 官方文档

requests.post("http://127.0.0.1:8080/setPower", json={"powerA": POWER, "powerB": POWER})

i = 0
while True:
    if i >= len(WAVES):
        i = 0
    WAVES[i]["channel"] = CHANNEL
    print(requests.post("http://127.0.0.1:8080/sendWave", json=WAVES[i]).text + "\n")
    time.sleep(0.1)
    i += 1
