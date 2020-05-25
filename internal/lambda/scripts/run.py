import gzip
import base64
import subprocess


def main(event, context):
    mb = subprocess.Popen(["./memorybox"]+event['args'][1:], env={
        "MEMORYBOX_LAMBDA_MODE": "true",
        "MEMORYBOX_CONFIG": event['config'],
    }, stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if "stdin" in event:
        stderr, stdout = mb.communicate(event["stdin"].encode())
    else:
        mb.stdin.close()
        mb.wait()
        stdout = mb.stdout.read()
        stderr = mb.stderr.read()
    return {
        "code": int(mb.returncode),
        "stdout": str(base64.b64encode(gzip.compress(stdout)).decode()),
        "stderr": str(base64.b64encode(gzip.compress(stderr)).decode())
    }
