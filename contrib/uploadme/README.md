Upload.me
===============
Small Golang HTTP-server that is tightly coupled
to the webbrowser for big file uploading.

How?
- XHR2
- ArrayBuffer.slice
- FileReader to read slices from FS

Notify rejected in browser?
In Chrome 62 and newer you cannot request notification api at all unless the site is https:// secured. (see issue 779612) If you do have https on the site you should be able to use notifications and background push notifications.

TODO
- Checksums?
- Limit amount of files created by hour?
