
# AYC
An automated transaction system for anarchy servers, powered by Baritone bots.

(psst... join our discord!)
<a href="https://discord.gg/ahdtzGS"><img src="https://www.themarysue.com/wp-content/uploads/2017/08/Screen-Shot-2017-08-14-at-4.33.47-PM.jpg" data-canonical-src="" width="200"></img></a>

## SETUP
First you'll need to clone the repo. On github press this button to get the clone URL / download as zip.
![Clone Repo Button on Github](https://image.prntscr.com/image/EhuFzx_dQvaeDVsek0lfUw.png)
Next, open terminal/command prompt and `cd` to the directory where you cloned the repo.
You probably don't have Golang installed on your computer, so please download it from [here](https://golang.org/dl/). This is what the backend was written in.

After going through the installer, you should be able to go to the cloned repo and do `go build`. You will probably receive an error, stating that `gcc is not defined`. For this, you will need to install gcc so Go can access the database.

Go [here](http://tdm-gcc.tdragon.net/download) to install `gcc`. If after this you still receive the error, you will need to install [Mingw Terminal
](https://sourceforge.net/projects/mingw-w64/), which allows you to run linux commands on Windows.

This backend relies on Discord bots, so for this you will need a discord auth token and discord auth secret in your PATH variables. Search online for an in depth tutorial on modifying the PATH variable on your system.

Once you have done this, you should be able to run the backend just fine. 
If you're using Linux or Mac do `go build && ./exchange`
If you're using Windows do `go build && exchange`

If everything goes well, go to `localhost:3000` and the website should load. If anything goes horribly wrong please create an issue, although do not expect a fast reply.