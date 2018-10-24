set -e
set -x
set -u

env GOOS=linux GOARCH=amd64 go build
echo -n $'\003' | dd bs=1 count=1 seek=7 conv=notrunc of=./hfast

echo "# scp -P2016 hfast root@104.248.142.101:~"
scp hfast mark@192.168.178.44:~
