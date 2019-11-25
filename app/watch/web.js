console.log('web ok');
var zWatchJs = [], zWatchCss = {}, zwatchWs;
(function () {
  document.addEventListener('DOMContentLoaded', function (event) {
    var a = document.createElement('a'), url = '';
    document.querySelectorAll('script').forEach(function (e) {
      if (!!e.src) {
        a.href = e.src;
        url = a.href;
        zWatchJs.push(url.replace(location.origin + '/', ''));
      }
    });
    document.querySelectorAll('link').forEach(function (e) {
      if (!!e.href) {
        a.href = e.href;
        zWatchCss[a.pathname.substring(1)] = e;
      }
    });
  });
  var __SpaInit__ = function () {
    try {
      var host = location.host;
      zwatchWs = new WebSocket('ws://' + host + '/');
      zwatchWs.onopen = function (evt) {};
      zwatchWs.onmessage = function (evt) {
        var j = JSON.parse(evt.data);
        console.log('update: ', j.Name);
        for (var i = 0, l = zWatchJs.length; i < l; i++) {
          if (zWatchJs[i] === j.Name) {
            location.reload();
            return;
          }
        }
        for (var k in zWatchCss) {
          if (k === j.Name) {
            if (zWatchCss.hasOwnProperty(k)) {
              var old = zWatchCss[k].href;
              zWatchCss[k].href = '';
              zWatchCss[k].href = old;
              return;
            }
          }
        }
        if (location.pathname === ('/' + j.Name)) {
          location.reload();
        }
      };
      zwatchWs.onclose = function (evt) {
        console.warn('Disconnect from zwatch.');
        setTimeout(function () {
          __SpaInit__();
        }, 300);
      };
    } catch (err) {}
  };
  __SpaInit__();
})();
