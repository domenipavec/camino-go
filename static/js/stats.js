google.load('visualization', '1.0', {'packages': ['corechart']});
google.setOnLoadCallback(displayStatsCharts);

function displayStatsCharts () {
    statsData.forEach(function (data) {
        var container = $(data.id);

        // Create the data table.
        var dataTable = new google.visualization.DataTable();
        dataTable.addColumn('number', 'Dan');
        dataTable.addColumn('number', data.title);
        dataTable.addColumn('number', 'Povprecje');

        var average = 0.0;
        data.data.forEach(function (value) {
            average += value;
        });
        average /= data.data.length;

        var i = 0;
        data.data.forEach(function (value) {
            i++;
            dataTable.addRow([{
                v: i,
                f: i + '. dan',
            }, {
                v: value,
                f: value.toFixed(1) + ' ' + data.unit,
            }, {
                v: average,
                f: average.toFixed(1) + ' ' + data.unit,
            }]);
        });

        // Set chart options
        var options = {
            hAxis: {
                title: 'Dan'
            },
            vAxis: {
                title: data.title + ' [' + data.unit + ']'
            },
            legend: {
                position: 'none'
            },
            chartArea: {
                left: '15%',
                width: '80%'
            },
            width: 100,
            height: 100
        };

        // Instantiate and draw our chart, passing in some options.
        var chart = new google.visualization.LineChart(container.get(0));
        chart.draw(dataTable, options);

        var resize = function () {
            container.height(container.width() * 0.75);
            options.width = container.width();
            options.height = container.width() * 0.75;
            options.chartArea.left = 45;
            options.chartArea.width = container.width() - options.chartArea.left - 10;
            options.chartArea.top = 10;
            options.chartArea.height = container.height() - options.chartArea.top - 30;
            chart.draw(dataTable, options);
        };

        resize();
        $(window).resize(resize);
    });
}
