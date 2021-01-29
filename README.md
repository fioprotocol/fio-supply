# minted

This is a microservice for getting current token state: total supply, circulating supply, locked tokens, bp bucket,
and bp rewards. By default it returns the response in whole FIO as a float with 9 digits of precision. The values
are calculated every 126 seconds (each full production round), and will return the previous calculation if there is
an error getting the values. The `X-Last-Refreshed:` header in the response has the time of the most recent refresh.

All heavy lifting is performed by the [fio-go](https://github.com/fioprotocol/fio-go/blob/master/locked-tokens.go) library.
Included is a [Postman collection](postman/) to assist in using the service.

The valid paths are:

 - `/supply` or `/minted` for current supply.
 - `/locked` for current count of locked tokens
 - `/bprewards`
 - `/bpbucket`
 - `/circulating` == supply - locked - bprewards - bpbucket

Modifiers:

 - Adding `/suf` to the path will return the value as an unsigned integer in smallest units instead of a float.
 - Adding `/int` to the path will return the value as an unsigned integer in whole FIO instead of a float.
 - Adding `?json=true` will return the result as a json object.

For example:

 - `https://fioprotocol.io/circulating`
 - `https://fioprotocol.io/supply/suf`
 - `https://fioprotocol.io/locked?json=true`
 - `https://fioprotocol.io/bpbucket/suf?json=true`
