import time
import sys
import MAX31855  # K-, J-, N-, T-, S-, R-, or E-type thermocouple (should be written on back side)

CLK = 11
CS = 10
DO = 9

sensor = MAX31855.MAX31855(CLK, CS, DO)
sys.stdout.write(str(round(sensor.readLinearizedTempC(),2)))
state = sensor.readState()
if state["fault"] == True:
    sys.stdout.write(str(state)+"\n")
sys.stdout.flush()
sys.exit(0)

# while True: #TODO: create sensor.service, starting observer.py loop, writing to a file. read file in go
#     sys.stdout.write(str(round(sensor.readLinearizedTempC(),2))+"\n") #sys.stdout.write(str(sensor.readTempC()))
#     sys.stdout.flush()
#     time.sleep(2)
