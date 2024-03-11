#!/bin/bash

main(){
    SCRIPTPATH=$(dirname $(readlink -f "$0"))
    cd $SCRIPTPATH
    if [ "${PWD##*/}" = "burnmaid" ]; then
        cd ..
    fi
    if [ ! -e burnmaid ]; then
        mkdir burnmaid
    fi
    cd burnmaid
    
    deps
    download
    install

    journalctl -f -u burnmaid

    exit
}

deps(){
    echo "install dependencies..."
    apt update
    apt install -y python3-pip gnuplot
    pip3 install rpi-rf RPi.GPIO
    raspi-config nonint do_i2c 0
    # python3 external/setup.py install #TODO: remove setup files
    # sudo iwconfig wlan0 power off
}

download(){
    version=$(curl -I https://github.com/martinbreu/burnmaid/releases/latest | awk -F '/' '/^location/ {print  substr($NF, 1, length($NF)-1)}')
    
    curl -L https://github.com/martinbreu/burnmaid/archive/refs/tags/${version}.zip --output ./burnmaid.zip
    unzip -o ./burnmaid.zip -d ./
    mv burnmaid-*/* .

    curl -L https://github.com/martinbreu/burnmaid/releases/download/${version}/burnmaid --output ./cmd/burnmaid
}

install(){
    echo "enable systemd service burnmaid"
    chmod +x $(pwd)/cmd/burnmaid
    bash -c 'echo "[Unit]
    Description=burnmaid
    [Service]
    WorkingDirectory=$(pwd)/cmd
    Type=simple
    Restart=always
    RestartSec=1
    ExecStart=$(pwd)/cmd/burnmaid
    [Install]
    WantedBy=multi-user.target" > /etc/systemd/system/burnmaid.service'
    systemctl daemon-reload
    systemctl enable burnmaid.service
    systemctl start burnmaid.service
    systemctl status burnmaid.service
}

main
