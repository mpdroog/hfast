"use strict";
var uniq = 0;
var whitelist = ['audio/wav', 'audio/mp3'];

function htmlEscape(str) {
  return str
  .replace(/&/g, '&amp;')
  .replace(/"/g, '&quot;')
  .replace(/'/g, '&#39;')
  .replace(/</g, '&lt;')
  .replace(/>/g, '&gt;');
}

function prep(files) {
  for (var i = 0, f; f = files[i]; i++) {
    console.log(f);
    if (whitelist.indexOf(f.type) === -1) {
    	output = '<li class="red"><strong><i class="fa fa-exclamation-triangle" aria-hidden="true"></i> Ignored: ' + htmlEscape(f.name) + '</strong>';
    	document.getElementById('list').children[0].innerHTML += output;
    	return;
    }

    var id = 'result_' + uniq;
    uniq++;

    chunkedReader(f, {
      chunkSize: 1 * 1024*1024, // 1mb
      id: id,
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
                return;
              }

              // (5/10)*100
              var percent = ((opts.idx+1) / opts.total) * 100;
              var $progress = document.getElementById(opts.id + '_step');
              if ($progress) {
                $progress.style.width = percent + '%';
                $progress.innerHTML = percent + '%';
              }
              // TODO: Update UI?
              if (percent == 100) {
                document.getElementById(opts.id).innerHTML = '<strong class="green"><i class="fa fa-check-circle" aria-hidden="true"></i> ' + htmlEscape(f.name) + '</strong>';
              }
              fnNext();
            } else {
              document.getElementById(opts.id).innerHTML = '<strong class="red"><i class="fa fa-exclamation" aria-hidden="true"></i> ' + htmlEscape(f.name) + '</strong>';
            }
          }
        };
        xhr.onerror = function () {
          document.getElementById(opts.id).innerHTML = '<strong class="red"><i class="fa fa-exclamation" aria-hidden="true"></i> ' + htmlEscape(f.name) + '</strong>';
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
      output += '<li><strong class="green"><i class="fa fa-check-circle" aria-hidden="true"></i> ' + htmlEscape(name) + '</strong></li>';
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