var zwatchWs,
  SpaForChildren,
  SpaResource = { sta: {}, mod: {} };
(function () {
  var sList = [],
    cList = [];
  document.querySelectorAll('link').forEach(e => {
    var c = e.href;
    c && cList.push([c.replace(location.origin, ''), e]);
  });
  document.querySelectorAll('script').forEach(e => {
    var s = e.src;
    s && sList.push(s.replace(location.origin, ''));
  });
  var inCss = function (href) {
    for (var c in cList) {
      if (cList.hasOwnProperty(c) && cList[c][0] === href) {
        cList[c][1].href = href + '?v=' + +new Date();
        return;
      }
    }
  };
  var inJs = function (src) {
    for (var s in sList) {
      if (sList.hasOwnProperty(s) && sList[s] === src) {
        location.reload();
        return;
      }
    }
  };
  var Resource = { sta: {}, mod: {} };
  var isOf = function (v) {
    if (v.indexOf('/') !== 0) {
      v = '/' + v;
    }
    return v;
  };
  var remove = function (mod) {
    var s = Resource.mod[mod];
    var sta = Resource.sta;
    delete Resource.mod[mod];
    if (!s) {
      return;
    }
    for (var v of s) {
      var _s = sta[v] || [];
      if (_s.length > 1) {
        var i = _s.indexOf(mod);
        _s.splice(i, 1);
      } else {
        delete sta[v];
      }
    }
  };
  var add = function (mod, re) {
    if (re === true) {
      re = SpaResource.mod[mod] || [];
    } else {
      SpaResource.mod[mod] = re;
    }
    Resource.mod[mod] = re;
    var sta = Resource.sta;
    var _s = {};
    for (var v of re) {
      v = isOf(v);
      var d = sta[v] || [];
      if (d.indexOf(mod) >= 0) continue;

      d.push(mod);
      _s[v] = d;
    }
    Object.assign(sta, _s);
    Object.assign(SpaResource.sta, _s);
  };
  var __SpaHot__ = function () {
    SpaForChildren = function ($children, r) {
      var len = $children.length;
      for (var i = 0; i < len; i++) {
        var el = $children[i];
        if (el && r) {
          setTimeout(function () {
            el.$vnode.context.$forceUpdate();
          });
        }
        SpaForChildren(el.$children, r);
      }
    };
    SpaForChildren(Spa.vue.$children, 1);
  };
  var __SpaInit__ = function () {
    try {
      var host = location.host;
      zwatchWs = new WebSocket('ws://' + host + '/');
      zwatchWs.onopen = function (evt) {};
      zwatchWs.onmessage = function (evt) {
        var j = JSON.parse(evt.data);
        console.log('update: ', j.Name);
        if (j.Name) {
          if (j.Name.indexOf('/') !== 0) {
            j.Name = '/' + j.Name;
          }
          var baseUrl = Spa.baseUrl;
          if (baseUrl.indexOf('/') !== 0) {
            baseUrl = '/' + baseUrl;
          }
          var name = new RegExp(baseUrl + '(.*)' + Spa.suffix, 'g').exec(
            j.Name
          );
          if (name) {
            Spa.loadMod(name[1], true).then(function () {
              __SpaHot__();
            });
          } else {
            var mod =
              SpaResource.sta[
                j.Name
                ];
            if (mod) {
              mod = mod.map(function (v) {
                return v.replace(/_/g, '/');
              });
              Spa.loadMod(mod, true).then(function () {
                __SpaHot__();
              });
            } else {
              inJs(j.Name);
              inCss(j.Name);
            }
          }
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
  window['_SpaModGet'] = (name, res) => {
    if (res === false) {
      remove(name);
    } else {
      add(name, res);
    }
  };
  if (window['Spa']) {
    __SpaInit__();
  } else {
    (function () {
      console.log('no vue-spa');
    })();
  }
})();
