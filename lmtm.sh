#!/bin/bash

#def var & defaults
l_port=${1:-4431}
#r_host=${2:-10.0.0.2}
#r_port=${3:-443}
#user=${5:-dato}

#read server ip
if [ -z "$4" ]; then
    echo "enter client public ip:"
    read -p "server (default: 102.217.231.243): " r_serv
    r_serv=${r_serv:-102.217.231.243}
else
    r_serv=$4
fi

#read remote host
if [ -z "$2" ]; then
    read -p "remote local (default: 10.0.0.2): " r_host
    r_host=${r_host:-10.0.0.2}
else
    r_host=$2
fi

# read port
if [ -z "$3" ]; then
    read -p "remote port (default: 443): " r_port
    r_port=${r_port:-443}
else
    r_port=$3
fi

#read user
if [ -z "$5" ]; then
    read -p "user (default: dato): " user
    user=${user:-dato}
else
    user=$5
fi


#print summery 
echo ""
echo "======================="
echo "+         LMTM        +"   
echo "======= summary ======="
echo "local port: $l_port"
echo "remote: $r_host:$r_port" 
echo "server: $r_serv"
echo "user: $user"
echo "======================="

#build command
ssh -oHostKeyAlgorithms=+ssh-rsa -L $l_port:$r_host:$r_port $user@$r_serv