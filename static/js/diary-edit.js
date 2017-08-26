/* global $,browser_key,google */

$(function () {
	$.getScript('//maps.googleapis.com/maps/api/js?key=' + browser_key + '&sensor=false&callback=initAutocomplete&libraries=places');
	$('#gps').change(function () {
		var value = $('#gps option:selected').first().attr('rel');
		if (value !== 'Neznano') {
			$('#city').val(value);
		}
	});
});

function initAutocomplete () {
	$('.google_autocomplete').each(function () {
		var ac = new google.maps.places.Autocomplete(this);
	});
}

$(document).ready(function() {
  $('#content').summernote({
	  height: 300,
	  toolbar: [
		  ['style', ['bold', 'italic', 'underline', 'clear']],
		  ['do', ['undo', 'redo']],
		  ['font', ['strikethrough', 'superscript', 'subscript']],
		  ['insert', ['link', 'video', 'table', 'hr']],
		  ['para', ['style', 'ul', 'ol']],
		  ['misc', ['fullscreen', 'codeview']],
	  ],
  });
});
