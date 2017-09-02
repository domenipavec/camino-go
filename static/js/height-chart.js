google.load('visualization', '1.0', {'packages': ['corechart']});
google.setOnLoadCallback(displayHeightChart);

function displayHeightChart () {
	var heightChart = $('#height-chart');

	// Create the data table.
	var dataTable = new google.visualization.DataTable();
	dataTable.addColumn('number', 'Razdalja');
	dataTable.addColumn('number', 'Višina');


    var decimation = Math.floor(heightChartData.length / 50);
    var average = 0;
    for (var i = 1; i < heightChartData.length; i++) {
        average += parseFloat(heightChartData[i].elevation)
        if (i % decimation !== 0) {
            continue;
        }
        average /= decimation;
        var value = heightChartData[i];
		if (value.elevation !== 0.0) {
			dataTable.addRow([{v: parseFloat(value.dist), f: parseFloat(value.dist).toFixed(1) + ' km'}, {v: average, f: average.toFixed(1) + ' m'}]);
		}

        average = 0;
	}

	// Set chart options
	var options = {
		hAxis: {
			title: 'Razdalja [km]'
		},
		vAxis: {
			title: 'Višina [m]'
		},
		legend: {
			position: 'none'
		},
		chartArea: {
			left: '15%',
			width: '85%'
		},
		width: 100,
		height: 100
	};

	// Instantiate and draw our chart, passing in some options.
	var chart = new google.visualization.LineChart(heightChart.get(0));
	chart.draw(dataTable, options);

	var resize = function () {
		heightChart.height(heightChart.width() * 0.75);
		options.width = heightChart.width();
		options.height = heightChart.width() * 0.75;
		options.chartArea.left = 45;
		options.chartArea.width = heightChart.width() - options.chartArea.left;
		options.chartArea.top = 10;
		options.chartArea.height = heightChart.height() - options.chartArea.top - 30;
		chart.draw(dataTable, options);
	};

	resize();
	$(window).resize(resize);
}
