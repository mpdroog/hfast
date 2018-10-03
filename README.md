HFast
-------------
HTTP-Server with convention over config.

Why not NGINx?
- No config, shaving off a lot of time
 configuring domains
- Wanted simple out of the box HTTPS/Ratelimit
- Easy experimentation as all is modifyable

So what the server offers:
- Built-in ratelimit of 30req/min from a specific IP
- FasctCGI to PHP

Future plan(s)
- Write small co-worker to offer distributed (DNS)
 hosting where the site is kept online when nodes fall off.

Thanks to:
* https://github.com/coreos/go-systemd/tree/master/examples/activation/httpserver

