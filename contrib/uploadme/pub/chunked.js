"use strict";
// Chunked reader is a simple wrapper around Filereader.slice
// to read (large) files in fixed chunks.
// https://stackoverflow.com/questions/14438187/javascript-filereader-parsing-long-file-in-chunks#28318964
function chunkedReader(file, opts) {
  var self = {
  	 chunkSize: opts.chunkSize,
  	 chunkCount: 0,
  	 offset: 0,
  	 file: file,
  	 r: new FileReader()
  };
  self.chunkCount = Math.ceil(file.size / self.chunkSize);
  self.r.onload = readEventHandler;
  opts.idx = opts.idx || 0;
  opts.total = self.chunkCount;
  console.log('chunkedReader file=' + file.name + ' size=' + self.file.size + ' chunks=' + self.chunkCount);

  function readEventHandler(evt) {
  	if (evt.target.error !== null) {
  		console.log("Read error: " + evt.target.error);
  		return;
  	}

  	var offend = self.offset + evt.target.result.byteLength;
	console.log('chunkedReader.chunk file=' + self.file.name + ' offset=' + self.offset + ' offend=' + offend);
	self.offset = offend;

	var fn = function(ok) {
		if (! ok) {
			console.log("chunkedReader.cancel");
			opts.fnDone(opts);
			return;
		}
		if (self.offset >= self.file.size) {
			console.log("chunkedReader.done");
			opts.fnDone(opts);
			return;
		}
		opts.idx++;
		chunkReaderBlock.bind(this)();
	}
	opts.fnChunk(file, evt.target.result, opts, fn.bind(self));
  }

  function chunkReaderBlock() {
  	console.log("chunkedReader.nextChunk from=" + this.offset + " to=" + (this.offset + this.chunkSize));
	var blob = this.file.slice(this.offset, this.offset + this.chunkSize);
	this.r.readAsArrayBuffer(blob);
  }
  chunkReaderBlock.bind(self)();
}

var limit = 4;
var queue = [];
var workers = 0;
function limitedChunkedReader(file, opts) {
	if (workers === limit) {
		// Wait!
		queue.push({file: file, opts: opts});
		return;
	}

	function next() {
		// Done handler
		// If anything is pending start that!
		if (queue.length === 0) {
			// Done, I quit
			workers--;
			return;
		}

		var job = queue.shift();
		job.opts.fnDone = next;
		chunkedReader(job.file, job.opts);
	}

	// Let it start
	workers++;
	opts.fnDone = next;
	chunkedReader(file, opts);
}