/* global $,google */

$.urlParam = function (name) {
  var results = new RegExp("[?&amp;]" + name + "=([^&amp;#]*)").exec(
    window.location.href
  );
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
  defaultIndex: 0,
};

// init marker groups
$(function () {
  $.getScript(
    "//maps.googleapis.com/maps/api/js?key=" +
      browser_key +
      "&sensor=false&callback=initializeMap&libraries=marker"
  );

  mapApp.defaultIndex = $.urlParam("index");
  mapApp.defaultMarker = $.urlParam("marker");
  mapApp.defaultPath = $.urlParam("path");
});

function fitToMarkers() {
  var bounds = new google.maps.LatLngBounds();
  $.each(mapApp.markers, function (i, markerGroup) {
    $.each(markerGroup, function (id, marker) {
      if (marker.map) {
        bounds.extend(marker.position);
      }
    });
  });

  if (!bounds.isEmpty()) {
    // Don't zoom in too far on only one marker
    if (bounds.getNorthEast().equals(bounds.getSouthWest())) {
      var extendPoint1 = new google.maps.LatLng(
        bounds.getNorthEast().lat() + 0.01,
        bounds.getNorthEast().lng() + 0.01
      );
      var extendPoint2 = new google.maps.LatLng(
        bounds.getNorthEast().lat() - 0.01,
        bounds.getNorthEast().lng() - 0.01
      );
      bounds.extend(extendPoint1);
      bounds.extend(extendPoint2);
    }

    mapApp.map.fitBounds(bounds);
  } else {
    mapApp.map.fitBounds(mapApp.fullBounds);
  }
}

// mouse events for polylines
function mouseOut(event) {
  this.setOptions({
    strokeOpacity: 0.5,
  });
}
function mouseIn(event) {
  this.setOptions({
    strokeOpacity: 1,
  });
}

// init markers and paths
function initializeMap() {
  var mapDiv = $(".map");
  mapDiv.show();
  mapDiv.height(mapDiv.width() * 0.75);
  $(window).resize(function () {
    mapDiv.height(mapDiv.width() * 0.75);
  });

  var mapOptions = {
    zoom: 8,
    mapTypeId: google.maps.MapTypeId.ROADMAP,
    mapId: "51e440109afe145b",
  };
  mapApp.map = new google.maps.Map(mapDiv.get(0), mapOptions);

  mapApp.fullBounds = new google.maps.LatLngBounds();

  $(".map-group").each(function () {
    var map_group = this;

    var index = parseInt($(map_group).attr("rel"), 10);
    if (mapApp.defaultIndex == null) {
      mapApp.defaultIndex = index;
    }

    var color = $(".map-color-icon", map_group).data("color");
    var colorPin = new google.maps.marker.PinElement({
      background: color,
      borderColor: color,
      glyphColor: "#dfdfdf",
      scale: 0.7,
    });
    var grayPin = new google.maps.marker.PinElement({
      background: "#dfdfdf",
      borderColor: "#000000",
      glyphColor: "#0000ff",
      scale: 0.7,
    });
    $(".map-color-icon", map_group).append(colorPin.element);
    $(".map-gray-icon", map_group).append(grayPin.element);
    if (index !== mapApp.defaultIndex) {
      $(".map-color-icon", map_group).hide();
    } else {
      $(".map-gray-icon", map_group).hide();
    }

    $("a", map_group).click(function () {
      $(".map-color-icon", map_group).toggle();
      $(".map-gray-icon", map_group).toggle();

      mapApp.closeWindows();

      if (!(index in mapApp.markers)) {
        loadData(index);
      } else {
        $.each(mapApp.markers[index], function (id, marker) {
          marker.map = marker.map ? null : mapApp.map;
        });
        $.each(mapApp.polylines[index], function (id, polyline) {
          polyline.setVisible(!polyline.getVisible());
        });

        fitToMarkers();
      }

      return false;
    });
  });

  loadData(mapApp.defaultIndex);
}

function loadData(index) {
  var isFit = false;
  $.ajax(`/map/group/${index}`).done(function (data) {
    var color = $(`#map-group-${index} .map-color-icon`).data("color");
    mapApp.markers[index] = {};
    mapApp.polylines[index] = {};

    $(data.entries).each(function (i, entry) {
      var content = `<strong>${entry.title}</strong>`;
      if (entry.diary) {
        if (entry.diary.image) {
          content += `<br>
<a href="/diary/${entry.diary.id}" title="${entry.diary.image.description}">
    <img src="https://res.cloudinary.com/dvmih7vrf/image/fetch/w_200,h_100,c_fill/${entry.diary.image.url}" alt="${entry.diary.image.description}">
</a>`;
        }
        content += `<br>Preberi v dnevniku: <a href="/diary/${entry.diary.id}">${entry.diary.title}</a>`;
      }
      if (entry.description) {
        content += `<br>${entry.description}`;
      }

      // construct marker
      var point = new google.maps.LatLng(entry.latitude, entry.longitude);
      var pin = new google.maps.marker.PinElement({
        background: color,
        borderColor: color,
        glyphColor: "#dfdfdf",
        scale: 0.7,
      });
      var marker = new google.maps.marker.AdvancedMarkerElement({
        position: point,
        map: mapApp.map,
        content: pin.element,
      });

      mapApp.fullBounds.extend(point);

      // store marker
      mapApp.markers[index][entry.id] = marker;

      // open bubble on click
      var infoWindow = null;
      var openFunction = function () {
        mapApp.closeWindows();
        if (!infoWindow) {
          // construct info bubble on demand
          var infoWindow = new google.maps.InfoWindow({
            content: content,
          });
          mapApp.infoWindows.push(infoWindow);
        }
        infoWindow.open(mapApp.map, marker);
        return false;
      };
      google.maps.event.addListener(marker, "click", openFunction);

      // show bubble for default marker
      if (entry.id === mapApp.defaultMarker) {
        openFunction();
      }

      // load gps if available
      if (entry.gps_id !== 0) {
        // parse points
        var path = [];
        var bounds = new google.maps.LatLngBounds();
        $.each(data.gps[entry.gps_id], function (i, value) {
          var point = new google.maps.LatLng(value.lat, value.lon);
          path.push(point);
          if (entry.gps_id === mapApp.defaultPath) {
            bounds.extend(point);
          }
        });

        // construct polyline
        var polyline = new google.maps.Polyline({
          path: path,
          geodesic: true,
          strokeColor: "#0000cc",
          strokeOpacity: 0.5,
        });

        // different color for even and odd ones
        if (i % 2) {
          polyline.setOptions({
            strokeColor: "#006600",
          });
        }

        // set map
        polyline.setMap(mapApp.map);

        mapApp.polylines[index][entry.id] = polyline;

        // fit map if on map id
        if (!bounds.isEmpty()) {
          mapApp.map.fitBounds(bounds);
          isFit = true;
        }

        // move marker to last gps point
        marker.position = path[path.length - 1];

        // set mouse over
        google.maps.event.addListener(polyline, "mouseover", mouseIn);
        google.maps.event.addListener(polyline, "mouseout", mouseOut);

        // set mouse click
        google.maps.event.addListener(polyline, "click", openFunction);
      }
    });

    if (!isFit) {
      fitToMarkers();
    }
  });
}
