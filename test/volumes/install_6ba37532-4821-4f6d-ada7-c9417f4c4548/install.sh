#!/bin/bash
echo $(pwd)
mkdir -p /mnt/data
cd /mnt/data
echo $(pwd)
apt-get update
npm init -y
echo "Hello World" > hello.txt
echo "console.log('abc');" > index.js