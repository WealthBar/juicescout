# JuiceScout

A script to migrate documentation from HelpJuice to HelpScout

## Setup
JuiceScout was thrown together quickly, so it's not the most user-friendly thing in the world.

It needs five things to be able to work:
 - A HelpScout API key
 - A HelpJuice categories.csv
 - A HelpJuice questions.csv
 - A HelpJuice answers.csv
 - A HelpJuice site name
 
HelpJuice provides you with an API key, but their API is new and broken and it just lets anyone grab EVERYTHING from their API without the key. Very secure.

To not rely on their API more than we have to, we use their exporter. You need to run these three commands for it:

`https://YOUR_SUBDOMAIN.helpjuice.com/api/export-all-categories` for categories.xls
`https://YOUR_SUBDOMAIN.helpjuice.com/api/export-all-questions` for questions.xls
`https://YOUR_SUBDOMAIN.helpjuice.com/api/export-all-answers` for answers.xls

Then using your favourite tool convert your XLS to CSV.

Dump your CSV in a `/data` directory in the JuiceScout directory. You can put it elsewhere, that's just where JuiceScout looks for it by default.

Speaking of, you need to setup the dotenv. Copy the `.env.example` to `.env` and put in your HelpScout API key next to `HELPSCOUT_API`

Once that's all done, `go run juicescout.go -j YOUR_SUBDOMAIN`

And bob's your uncle. Hopefully.
