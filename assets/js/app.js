(function() {
	$('#query').submit(function(e) {
		e.preventDefault();
		$.getJSON('api/search?q=' + $('#q').val(), function(data) {
			var result = $('#result');
			result.empty();
			if (!data || !data.length) {
				result.append('<li>No match is found.</li>');
			} else {
				data.forEach(function(hit) {
					console.log(hit);
					result.append('<li>' + hit + '</li>');
				});
			}
		}).fail(function() {
			var result = $('#result');
			result.empty();
			result.append('<li>Internal error captured and will be fixed soon.</li>');
		});
		return false;
	});
})(window.Zepto);
