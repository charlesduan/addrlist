module github.com/charlesduan/addrlist

go 1.13

require (
	git.sr.ht/~sircmpwn/aerc v0.0.0-20201121144050-67923707ffd8
	git.sr.ht/~sircmpwn/getopt v0.0.0-20191230200459-23622cc906b3
	github.com/emersion/go-message v0.13.1-0.20201112194930-f77964fe28bd
	github.com/go-ini/ini v1.52.0
	github.com/kyoh86/xdg v1.2.0
	github.com/mattn/go-isatty v0.0.12
	github.com/mitchellh/go-homedir v1.1.0
)

replace golang.org/x/crypto => github.com/ProtonMail/crypto v0.0.0-20200420072808-71bec3603bf3

replace github.com/gdamore/tcell => git.sr.ht/~sircmpwn/tcell v0.0.0-20190807054800-3fdb6bc01a50
