# Buttercup to Pass

This a small liter helper to migrate your Buttercup passwords to Pass.

The tool will only convert the Buttercup CSV format into a pass (and browserpass) compatible structure into your `.password-store` folder and it'll call the command line GPG tool to encrypt the data. 

## Usage

- You should have a pre initialised and working pass/gopass with configured GPG. (It should not ask for a password, use an agent.) 
- Export your passwords from Buttercup to CSV
- Run this tool `go run main.go -file <yourfile.csv>`
- Sync your store and you are done!


## Details

There's a `-dryrun` which won't do the conversion just print out the results to see what will happen.

The script will try to keep the category you had in the path and try to use the site URL as the filename, for example:

If there's a kickstarter.com row in your buttercup csv it'll output the following file:
```
~/.password-store/general/www.kickstarter.com
```

With the content:

```
<your password>
title: kickstarter.com
password: <your password>
login: <your username>
url: https://www.kickstarter.com/
group: general
comments:
```