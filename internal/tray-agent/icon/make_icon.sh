#/bin/sh

if [ "$#" -ne 2 ]; then
    echo "usage: $0 {image.png} {IconName}"
fi

if [ -z "$GOPATH" ]; then
    echo GOPATH environment variable not set
    exit
fi

if [ ! -e "$GOPATH/bin/2goarray" ]; then
    echo "Installing 2goarray..."
    go get github.com/cratonica/2goarray
    if [ $? -ne 0 ]; then
        echo Failure executing go get github.com/cratonica/2goarray
        exit
    fi
fi

if [ -z "$1" ]; then
    echo Please specify a PNG file
    exit
fi

if [ ! -f "$1" ]; then
    echo $1 is not a valid file
    exit
fi    



# $1=png , $2=varName

cat "$1" | $GOPATH/bin/2goarray $2 icon > $2.go
if [ $? -ne 0 ]; then
    echo Failure generating 
    exit
fi
echo Finished
