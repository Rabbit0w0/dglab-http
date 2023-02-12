import time

import requests

POWER = 55
CHANNEL = 1

WAVES = [{"paramX": 1, "paramY": 9, "paramZ": 0}, {"paramX": 1, "paramY": 9, "paramZ": 5},
         {"paramX": 1, "paramY": 9, "paramZ": 0}, {"paramX": 1, "paramY": 9, "paramZ": 10},
         {"paramX": 1, "paramY": 9, "paramZ": 0}, {"paramX": 1, "paramY": 9, "paramZ": 14},
         {"paramX": 1, "paramY": 9, "paramZ": 0}, {"paramX": 1, "paramY": 9, "paramZ": 17},
         {"paramX": 1, "paramY": 9, "paramZ": 0}, {"paramX": 1, "paramY": 9, "paramZ": 20},
         {"paramX": 1, "paramY": 9, "paramZ": 0}]  # 按捏渐强

POWER *= 7  # 官方文档

requests.post("127.0.0.1:8080/setPower", {"powerA": POWER, "powerB": POWER})

i = 0
while True:
    if i >= len(WAVES):
        i = 0
    WAVES[0]["channel"] = CHANNEL
    requests.post("127.0.0.1:8080/sendWave", WAVES[i])
    time.sleep(0.1)
    i += 1
