#!/usr/local/bin/python

import argparse
import signal
import sys
import time
import json

from rpi_rf import RFDevice

rfdevice = None

# pylint: disable=unused-argument


def exithandler(signal, frame):
    rfdevice.cleanup()
    sys.exit(0)


parser = argparse.ArgumentParser(
    description='Receives a decimal code via a 433/315MHz GPIO device')
parser.add_argument('-g', dest='gpio', type=int, default=27,
                    help="GPIO pin (Default: 27)")
args = parser.parse_args()

signal.signal(signal.SIGINT, exithandler)
rfdevice = RFDevice(args.gpio)
rfdevice.enable_rx()
timestamp = None


def get_most_frequent(List):
    counter = 0
    result = "No Signal"
    for i in List:
        curr_frequency = List.count(i)
        if (curr_frequency > counter):
            counter = curr_frequency
            result = i
    return result


codeList = []
pulseLengthList = []
protocolIndexList = []
t_end = time.time() + 60
while time.time() < t_end:
    time.sleep(0.01)
    if rfdevice.rx_code_timestamp != timestamp:
        timestamp = rfdevice.rx_code_timestamp
        codeList.append(str(rfdevice.rx_code))
        pulseLengthList.append(str(rfdevice.rx_pulselength))
        protocolIndexList.append(str(rfdevice.rx_proto))
        if len(protocolIndexList) > 10:
            t_end = time.time()

rfdevice.cleanup()
data = {
    "Code": get_most_frequent(codeList),
    "PulseLength": get_most_frequent(pulseLengthList),
    "ProtocolIndex": get_most_frequent(protocolIndexList),
}
sys.stdout.write(json.dumps(data))
sys.stdout.flush()
sys.exit(0)
