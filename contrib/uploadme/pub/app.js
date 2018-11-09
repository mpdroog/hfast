"use strict";
var uniq = 0;
var whitelist = ['audio/wav', 'audio/mp3'];
//var notify = false;

function htmlEscape(str) {
  return str
  .replace(/&/g, '&amp;')
  .replace(/"/g, '&quot;')
  .replace(/'/g, '&#39;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;');
}

function done() {
  console.log("done", window.notify);
  if (window.notify) {
    console.log("Show notify");
    var notification = new Notification("Finished uploading all assets.");
  }
}

function prep(files) {
  for (var i = 0, f; f = files[i]; i++) {
    console.log(f);
    if (whitelist.indexOf(f.type) === -1) {
    	var output = '<li class="red"><strong><i class="fa fa-exclamation-triangle" aria-hidden="true"></i> Ignored: ' + htmlEscape(f.name) + '</strong>';
    	document.getElementById('list').children[0].innerHTML += output;
    	return;
    }

    var id = 'result_' + uniq;
    uniq++;

    limitedChunkedReader(f, {
      chunkSize: 1 * 1024*1024, // 1mb
      id: id,
      chunksDone: done,
      fnChunk: function(f, bin, opts, fnNext) {
        var xhr = new XMLHttpRequest();
        xhr.open('POST', '/action/uploads/chunk?f='+ encodeURIComponent(f.name) + '&i=' + opts.idx + "&total=" + opts.total, true);
        xhr.setRequestHeader('Content-Type', 'application/octet-stream');
        xhr.onload = function(e) {
          // TODO: Error handle?
          if (xhr.readyState == 4) { 
            if (this.status == 200) {
              console.log("xhr res=", this.responseText);
              if (this.responseText !== "OK.") {
                document.getElementById(opts.id).innerHTML = '<strong class="red"><i class="fa fa-exclamation" aria-hidden="true"></i> ' + htmlEscape(f.name) + '</strong>';
                fnNext(false);
                return;
              }

              // (5/10)*100
              var percent = ((opts.idx+1) / opts.total) * 100;
              var $progress = document.getElementById(opts.id + '_step');
              if ($progress) {
                $progress.style.width = percent + '%';
                $progress.innerHTML = percent + '%';
              }
              if (percent == 100) {
                document.getElementById(opts.id).innerHTML = '<strong class="green"><i class="fa fa-check-circle" aria-hidden="true"></i> ' + htmlEscape(f.name) + '</strong>';
              }
              fnNext(true);
            } else {
              document.getElementById(opts.id).innerHTML = '<strong class="red"><i class="fa fa-exclamation" aria-hidden="true"></i> ' + htmlEscape(f.name) + '</strong>';
              fnNext(false);
            }
          }
        };
        xhr.onerror = function () {
          document.getElementById(opts.id).innerHTML = '<strong class="red"><i class="fa fa-exclamation" aria-hidden="true"></i> ' + htmlEscape(f.name) + '</strong>';
          fnNext(false);
        };
        xhr.send(bin);
      }
    });

    document.getElementById('list').children[0].innerHTML += '<li id=' + id + '><strong><i class="fa fa-circle-o-notch fa-spin fa-fw" aria-hidden="true"></i> ' + htmlEscape(f.name) + '</strong> <div class="progress_bar"><div id="' + id +'_step" class="percent">0%</div></div></li>';
  }
}

function handleFileSelect(evt) {
  evt.stopPropagation();
  evt.preventDefault();

  prep(evt.dataTransfer.files);
}
function handleFileChange(evt) {
  prep(evt.target.files);
}

function handleFileClick(evt) {
	document.getElementById('files').click();
}
function handleDragOver(evt) {
  evt.stopPropagation();
  evt.preventDefault();
  evt.dataTransfer.dropEffect = 'copy'; // Explicitly show this is a copy.
}
function handleNotify(e) {
  var checked = e.target.checked;
  if (! checked) {
    console.log("Disable notify");
    window.notify = checked;
  } else {
    if (!("Notification" in window)) {
      alert("This browser does not support desktop notification");
    // Let's check whether notification permissions have already been granted
    } else if (Notification.permission === "granted") {
      // All set
      console.log("Enable notify");
      window.notify = true;
    } else {
      // Ask
      Notification.requestPermission().then(function (permission) {
        // If the user accepts, let's create a notification
        if (permission === "granted") {
          console.log("Enable notify");
          window.notify = true;
        } else {
          alert("Desktop notification are rejected by you.");
        }
      });
    }
  }
}

/*  init */
var xhr = new XMLHttpRequest();
xhr.open('GET', '/action/uploads', true);
xhr.onload = function(e) {
  // TODO: Error handle?
  if (this.status == 200) {
    console.log("xhr.uploads", this.responseText);
    var obj = JSON.parse(this.responseText);
    var output = '';
    for (var i = 0; i < obj.length; i++) {
      var name = obj[i];
      output += '<li><strong class="green"><i class="fa fa-check-circle" aria-hidden="true"></i> ' + htmlEscape(name) + '</strong> <a href="#" class"js-del" data-idx="'+name+'"><i class="fa fa-trash"></i></a></li>';
    }
    document.getElementById('list').innerHTML = '<ul>' + output + '</ul>';
  }
};
xhr.send();

// Setup the listeners.
var dropZone = document.getElementById('drop_zone');
dropZone.addEventListener('dragover', handleDragOver, false);
dropZone.addEventListener('drop', handleFileSelect, false);
dropZone.addEventListener('click', handleFileClick, false);
document.getElementById('files').addEventListener('change', handleFileChange, false);
document.getElementById('js-notify').addEventListener('change', handleNotify, false);

document.getElementById("list").addEventListener("click", function(e) {
  if (e.target.nodeName === "I" && e.target.parentNode.dataset.idx) {
    var n = e.target.parentNode;
    window.upload = {n: n};
    console.log("Delete " + n.dataset.idx);

    var xhr = new XMLHttpRequest();
    xhr.open('GET', '/action/uploads/rm?f=' + n.dataset.idx, true);
    xhr.onload = function(ev) {
      // TODO: Error handle?
      if (this.status == 200) {
        var n = window.upload.n.parentNode;
        n.parentNode.removeChild(n);
      }
    };
    xhr.send();
  }
});
