# package icon

This package provides a set of icons that can be displayed in the 
system tray bar using the [**Indicator**](https://github.com/liqoTech/liqo/internal/tray-agent/app-indicator/Indicator) type.

- package files are automatically generated with the help of the 
[2goarray](https://github.com/cratonica/2goarray) tool that converts PNG 
images into **Golang []byte**. This is the format required by the 
[systray](https://github.com/getlantern/systray) package the **Indicator** exploits.

  - PNG source files are located under **assets/tray-agent/icons/tray-bar/**

> ```bash
>$GOPATH/bin/2goarray <IcoVarName> icon < <myimage>.png > <IcoVarName>.go
> ```

- a simple script ```make_icon.sh``` is already provided to ease the 
creation process. Execute this script to generate all the package files.
> ```bash
> # EXECUTE THE SCRIPT FROM THE PROJECT ROOT
> $ ./scripts/tray-agent/make_icon.sh
> ```

#### NOTE:
Given the small size of the icon shown in the tray bar, it is advisable to start 
from a picture of approximately 64x64 px 
or less, in order to reduce the amount of space.
