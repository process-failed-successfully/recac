import json

def parse(filename):
    fails = []
    with open(filename, 'r') as f:
        for line in f:
            if 'FAIL\t' in line:
                print(line.strip())

parse('cmd_tests.log')
