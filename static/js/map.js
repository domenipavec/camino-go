/* global $,google */

$.urlParam = function (name) {
    var results = new RegExp('[\?&amp;]' + name + '=([^&amp;#]*)').exec(window.location.href);
    if (results) {
        return parseInt(results[1], 10) || null;
    } else {
        return null;
    }
};

var mapApp = {
    infoWindows: [],
    markers: {},
    closeWindows: function () {
        $(mapApp.infoWindows).each(function (index, value) {
            value.close();
        });
    },
    polylines: {},
    defaultIndex: 0
};

// init marker groups
$(function () {
    $.getScript('//maps.googleapis.com/maps/api/js?key=' + browser_key + '&sensor=false&callback=initializeMap');

    mapApp.defaultIndex = $.urlParam('index');
    mapApp.defaultMarker = $.urlParam('marker');
    mapApp.defaultPath = $.urlParam('path');

    $('.map-group').each(function () {
        var map_group = this;

        var index = parseInt($('.map-list', map_group).attr('rel'), 10);
        if (mapApp.defaultIndex == null) {
            mapApp.defaultIndex = index;
        }

        if (index !== mapApp.defaultIndex) {
            $('.map-color-icon', map_group).hide();
            $('.map-gray-icon', map_group).css('display', 'inline');
        } else {
            $('.map-gray-icon', map_group).css('display', 'inline').hide();
        }
        mapApp.markers[index] = {};
        mapApp.polylines[index] = {};

        $('a', map_group).click(function () {
            $('.map-color-icon', map_group).toggle();
            $('.map-gray-icon', map_group).toggle();

            mapApp.closeWindows();

            $.each(mapApp.markers[index], function (id, marker) {
                marker.setVisible(!marker.getVisible());
            });
            $.each(mapApp.polylines[index], function (id, polyline) {
                polyline.setVisible(!polyline.getVisible());
            });

            fitToMarkers();

            return false;
        });
    });
});

function fitToMarkers () {
    var bounds = new google.maps.LatLngBounds();
    $.each(mapApp.markers, function (i, markerGroup) {
        $.each(markerGroup, function (id, marker) {
            if (marker.getVisible()) {
                bounds.extend(marker.position);
            }
        });
    });

    if (!bounds.isEmpty()) {
        // Don't zoom in too far on only one marker
        if (bounds.getNorthEast().equals(bounds.getSouthWest())) {
            var extendPoint1 = new google.maps.LatLng(bounds.getNorthEast().lat() + 0.01, bounds.getNorthEast().lng() + 0.01);
            var extendPoint2 = new google.maps.LatLng(bounds.getNorthEast().lat() - 0.01, bounds.getNorthEast().lng() - 0.01);
            bounds.extend(extendPoint1);
            bounds.extend(extendPoint2);
        }

        mapApp.map.fitBounds(bounds);
    } else {
        mapApp.map.fitBounds(mapApp.fullBounds);
    }
}

// mouse events for polylines
function mouseOut (event) {
    this.setOptions({
        strokeOpacity: 0.5
    });
}
function mouseIn (event) {
    this.setOptions({
        strokeOpacity: 1
    });
}

// init markers and paths
function initializeMap () {
    var mapDiv = $('.map');
    mapDiv.show();
    mapDiv.height(mapDiv.width() * 0.75);
    $(window).resize(function () {
        mapDiv.height(mapDiv.width() * 0.75);
    });

    var mapOptions = {
        zoom: 8,
        mapTypeId: google.maps.MapTypeId.ROADMAP
    };
    mapApp.map = new google.maps.Map(mapDiv.get(0), mapOptions);

    mapApp.fullBounds = new google.maps.LatLngBounds();

    var isFit = false;

    // iterate over markers
    $('.map-group li').each(function (i) {
        var groupIndex = parseInt($(this).closest('ul').attr('rel'), 10);
        var markerId = parseInt($(this).attr('data-id'), 10);

        // construct info bubble
        var infoWindow = new google.maps.InfoWindow({
            content: '<div>'.concat($(this).html()).replace(/ - /g, '<br />').concat('</div>')
        });
        mapApp.infoWindows.push(infoWindow);

        // construct marker
        var point = new google.maps.LatLng($(this).attr('data-lat'), $(this).attr('data-lon'));
        var marker = new google.maps.Marker({
            position: point,
            map: mapApp.map,
            icon: $(this).parents('div.map-group').find('img').attr('src')
        });

        // show default markers
        if (groupIndex !== mapApp.defaultIndex) {
            marker.setVisible(false);
        }
        mapApp.fullBounds.extend(point);

        // store marker
        mapApp.markers[groupIndex][markerId] = marker;

        // open bubble on click
        var openFunction = function () {
            mapApp.closeWindows();
            infoWindow.open(mapApp.map, marker);
            return false;
        };
        google.maps.event.addListener(marker, 'click', openFunction);

        // show bubble for default marker
        if (markerId === mapApp.defaultMarker) {
            openFunction();
        }

        // load gps if available
        var gpsId = parseInt($(this).attr('data-gps'), 10);
        if (gpsId !== 0) {
            // parse points
            var path = [];
            var bounds = new google.maps.LatLngBounds();
            $.each(gpsData[gpsId], function (i, value) {
                var point = new google.maps.LatLng(value.lat, value.lon);
                path.push(point);
                if (gpsId === mapApp.defaultPath) {
                    bounds.extend(point);
                }
            });

            // construct polyline
            var polyline = new google.maps.Polyline({
                path: path,
                geodesic: true,
                strokeColor: '#0000cc',
                strokeOpacity: 0.5
            });

            // different color for even and odd ones
            if (i % 2) {
                polyline.setOptions({
                    strokeColor: '#006600'
                });
            }

            // set map
            polyline.setMap(mapApp.map);

            if (groupIndex !== mapApp.defaultIndex) {
                polyline.setVisible(false);
            }

            mapApp.polylines[groupIndex][markerId] = polyline;

            // fit map if on map id
            if (!bounds.isEmpty()) {
                mapApp.map.fitBounds(bounds);
                isFit = true;
            }

            // move marker to last gps point
            marker.setPosition(path[path.length - 1]);

            // set mouse over
            google.maps.event.addListener(polyline, 'mouseover', mouseIn);
            google.maps.event.addListener(polyline, 'mouseout', mouseOut);

            // set mouse click
            google.maps.event.addListener(polyline, 'click', openFunction);
        }
    });

    if (!isFit) {
        fitToMarkers();
    }
}
