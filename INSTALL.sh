#!/bin/bash

# version="$1"
main(){
    SCRIPTPATH=$(dirname $(readlink -f "$0"))
    cd $SCRIPTPATH
    
    if [ ! -e burnmaid ]; then #TODO: and not allready in burnmaid
        mkdir burnmaid
        cd burnmaid
    fi
    
    deps
    download
    install
    
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
    #TODO: get latest?!, see page
    curl -L https://github.com/martinbreu/burnmaid/archive/refs/tags/${version}.zip --output ../burnmaid.zip
    curl -L https://github.com/martinbreu/burnmaid/releases/download/${version}/burnmaid --output ../cmd/burnmaid
    unzip -o ../burnmaid.zip -d ../
    rm ../burnmaid.zip
    sudo rsync --archive --remove-source-files ../burnmaid-* ../
}

install(){
    echo "enable systemd service burnmaid"
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
}

main
