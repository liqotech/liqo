#package icon

This package provides a set of icons that can be displayed in the 
system tray bar using the **app_indicator/Indicator** type.

- package files are automatically generated with the 
[2goarray](https://github.com/cratonica/2goarray) tool that converts PNG 
images into **[]byte**. This is the format required by the 
[systray](https://github.com/getlantern/systray) package exploited 
by **Indicator**.

> ```bash
>$GOPATH/bin/2goarray <IcoVarName> icon < myimage.png > <IcoVarName>.go
> ```

- a simple script ```make_icon.sh``` is already provided to easy the 
creation process:
> ```bash
> ./make_icon.sh <image.png> <IcoVarName>
> ```

####NOTE:
Given the small size of icon shown in the tray bar, it is advisable to start 
from a picture of approximately 64x64 px 
or less, in order to reduce the amount of space.
