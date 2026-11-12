/* @ds-bundle: {"format":4,"namespace":"RocketScience","components":[{"name":"Button"},{"name":"Switch"},{"name":"SegmentedControl"},{"name":"IgnitionButton"},{"name":"TelemetryStat"},{"name":"TelemetryHistory"},{"name":"LatencyBadge"},{"name":"ProxyNode"},{"name":"ProxyGroup"},{"name":"Trajectory"},{"name":"ConnectionTable"},{"name":"LogStream"},{"name":"ProfileCard"},{"name":"SearchField"},{"name":"Sidebar"},{"name":"TrayIndicator"},{"name":"MenuBarPopover"},{"name":"Tour"},{"name":"FirstRunCard"},{"name":"ConnectivityCheck"},{"name":"Flag"},{"name":"ProfileEditor"},{"name":"Sheet"},{"name":"TextField"},{"name":"Select"},{"name":"CodeEditor"},{"name":"OverrideGroup"},{"name":"RuleGuide"},{"name":"ContextMenu"},{"name":"CheckPopover"},{"name":"SettingsSection"},{"name":"SettingRow"},{"name":"SettingField"},{"name":"ListField"},{"name":"AddRuleSheet"},{"name":"Icon"}]} */
(function () {
  var React = window.React, h = React.createElement;
  var useState = React.useState, useMemo = React.useMemo, useRef = React.useRef, useEffect = React.useEffect, useLayoutEffect = React.useLayoutEffect;

  function cx() { return Array.prototype.filter.call(arguments, Boolean).join(' '); }
  function clamp(x, a, b) { return Math.max(a, Math.min(b, x)); }

  /* ---------- physics: mass / stiffness / damping, no durations ---------- */

  var springs = {
    snap: { mass: 1, stiffness: 520, damping: 38 },
    glide: { mass: 1, stiffness: 180, damping: 24 },
    heavy: { mass: 3, stiffness: 420, damping: 54 },
    orbit: { mass: 1, stiffness: 90, damping: 9 }
  };

  function reducedMotion() {
    return !!(window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
  }

  // A one-dimensional body on a spring. Semi-implicit Euler at a fixed 1/240 s step.
  function Body(x, spring, eps) {
    this.x = x; this.v = 0; this.target = x;
    this.spring = spring || springs.snap;
    this.eps = eps || 0.001;
    this.raf = 0; this.onFrame = null;
  }
  Body.prototype.step = function (dt) {
    var s = this.spring;
    var force = -s.stiffness * (this.x - this.target) - s.damping * this.v;
    this.v += force / s.mass * dt;
    this.x += this.v * dt;
  };
  Body.prototype.emit = function () { if (this.onFrame) this.onFrame(this.x, this.v); };
  Body.prototype.set = function (x) { this.stop(); this.x = x; this.target = x; this.v = 0; this.emit(); };
  Body.prototype.to = function (target, opts) {
    opts = opts || {};
    this.target = target;
    if (opts.velocity != null) this.v = opts.velocity;
    if (opts.spring) this.spring = opts.spring;
    if (reducedMotion()) { this.set(target); return; }
    this.start();
  };
  Body.prototype.start = function () {
    if (this.raf) return;
    var self = this, last = performance.now(), acc = 0, DT = 1 / 240;
    function tick(now) {
      acc += Math.min(0.064, (now - last) / 1000); last = now;
      while (acc >= DT) { self.step(DT); acc -= DT; }
      if (Math.abs(self.v) < self.eps * 10 && Math.abs(self.x - self.target) < self.eps) {
        self.x = self.target; self.v = 0; self.raf = 0; self.emit(); return;
      }
      self.emit();
      self.raf = requestAnimationFrame(tick);
    }
    this.raf = requestAnimationFrame(tick);
  };
  Body.prototype.stop = function () { if (this.raf) cancelAnimationFrame(this.raf); this.raf = 0; };

  // Where a released body will come to rest if left alone (for flick-to-snap).
  function project(x, v, spring) { return x + v * (spring.mass / spring.damping); }

  // Bake a spring into a CSS linear() easing and the duration it needs.
  function handoff(spring, samples) {
    samples = samples || 28;
    var b = new Body(0, spring, 0.001), t = 0, dt = 1 / 2400, xs = [];
    b.target = 1;
    while (t < 4) {
      b.step(dt); t += dt; xs.push(b.x);
      if (t > 0.05 && Math.abs(b.x - 1) < 0.001 && Math.abs(b.v) < 0.01) break;
    }
    var pts = [0];
    for (var i = 1; i < samples; i++) pts.push(Math.round(xs[Math.floor(xs.length * i / samples)] * 1000) / 1000);
    pts.push(1);
    return { easing: 'linear(' + pts.join(', ') + ')', duration: Math.round(t * 1000) };
  }

  function useBody(initial, spring, onFrame, eps) {
    var ref = useRef(null);
    if (!ref.current) ref.current = new Body(initial, spring, eps);
    ref.current.onFrame = onFrame;
    useEffect(function () { var b = ref.current; return function () { b.stop(); }; }, []);
    return ref.current;
  }

  function usePress(spring) {
    var el = useRef(null);
    var body = useBody(1, spring || springs.snap, function (x) {
      if (el.current) el.current.style.transform = x === 1 ? '' : 'scale(' + x + ')';
    }, 0.0005);
    return {
      ref: el,
      down: function () { body.to(0.96); },
      up: function () { body.to(1); }
    };
  }

  /* ---------- formatting ---------- */

  var fmt = {
    speed: function (bps) {
      if (bps == null || isNaN(bps)) return { value: '—', unit: '' };
      var u = ['B/s', 'KB/s', 'MB/s', 'GB/s'], i = 0;
      while (bps >= 1000 && i < u.length - 1) { bps /= 1024; i++; }
      return { value: bps >= 100 || i === 0 ? String(Math.round(bps)) : bps.toFixed(1), unit: u[i] };
    },
    bytes: function (b) {
      var u = ['B', 'KB', 'MB', 'GB', 'TB'], i = 0;
      while (b >= 1000 && i < u.length - 1) { b /= 1024; i++; }
      return (b >= 100 || i === 0 ? Math.round(b) : b.toFixed(1)) + ' ' + u[i];
    },
    clock: function (seconds) {
      var s = Math.max(0, Math.floor(seconds));
      function p(n) { return (n < 10 ? '0' : '') + n; }
      return 'T+' + p(Math.floor(s / 3600)) + ':' + p(Math.floor(s / 60) % 60) + ':' + p(s % 60);
    }
  };

  /* ---------- Icon ---------- */

  var ICONS = {
    sun: [['circle', 8, 8, 2.8], ['path', 'M8 1.5v1.6M8 12.9v1.6M1.5 8h1.6M12.9 8h1.6M3.4 3.4l1.1 1.1M11.5 11.5l1.1 1.1M3.4 12.6l1.1-1.1M11.5 4.5l1.1-1.1']],
    moon: [['path', 'M13.2 9.8A5.6 5.6 0 0 1 6.2 2.8a5.6 5.6 0 1 0 7 7z']],
    contrast: [['circle', 8, 8, 5.8], ['path', 'M8 2.2v11.6a5.8 5.8 0 0 0 0-11.6z', 'fill']],
    rocket: [['path', 'M8 1.5c2.2 1.4 3.3 3.6 3.3 6.2V11H4.7V7.7c0-2.6 1.1-4.8 3.3-6.2z'], ['path', 'M4.7 8.8 2.5 11v2.5l2.2-1.3M11.3 8.8l2.2 2.2v2.5l-2.2-1.3M6.5 13.5 8 15l1.5-1.5'], ['circle', 8, 6.4, 1.2]],
    orbit: [['circle', 8, 8, 2], ['ellipse', 8, 8, 6.5, 2.6, -30]],
    globe: [['circle', 8, 8, 6.5], ['path', 'M1.5 8h13M8 1.5c1.8 1.8 2.7 4 2.7 6.5S9.8 12.7 8 14.5C6.2 12.7 5.3 10.5 5.3 8S6.2 3.3 8 1.5z']],
    route: [['circle', 3.5, 12.5, 1.5], ['circle', 12.5, 3.5, 1.5], ['path', 'M5 12.5h4.5a2.5 2.5 0 0 0 0-5h-3a2.5 2.5 0 0 1 0-5H11']],
    pulse: [['path', 'M1.5 8h3L6 4l3 8 1.5-4h4']],
    terminal: [['rect', 1.5, 2.5, 13, 11, 2], ['path', 'M4.5 6l2 2-2 2M8.5 10h3']],
    layers: [['path', 'M8 2l6.5 3.5L8 9 1.5 5.5z'], ['path', 'M1.5 8.5 8 12l6.5-3.5M1.5 11.5 8 15l6.5-3.5']],
    sliders: [['path', 'M2.5 4.5h7M12.5 4.5h1M2.5 11.5h1M6.5 11.5h7'], ['circle', 11, 4.5, 1.5], ['circle', 5, 11.5, 1.5]],
    'alert-circle': [['circle', 8, 8, 6.5], ['path', 'M8 4.8v4M8 11.6v.1']],
    'alert-triangle': [['path', 'M7.13 2.5a1 1 0 0 1 1.74 0l5.86 10.14A1 1 0 0 1 13.86 14H2.14a1 1 0 0 1-.87-1.36L7.13 2.5z'], ['path', 'M8 6v3.5M8 11.6v.1']],
    'x-circle': [['circle', 8, 8, 6.5], ['path', 'M5.8 5.8l4.4 4.4M10.2 5.8l-4.4 4.4']],
    bolt: [['path', 'M9 1.5 3.5 9H8l-1 5.5L12.5 7H8z']],
    shield: [['path', 'M8 1.5l5.5 2v4c0 3.3-2.3 5.8-5.5 7-3.2-1.2-5.5-3.7-5.5-7v-4z']],
    'arrow-up': [['path', 'M8 13V3M4 7l4-4 4 4']],
    'arrow-down': [['path', 'M8 3v10M4 9l4 4 4-4']],
    refresh: [['path', 'M13.5 8a5.5 5.5 0 1 1-1.6-3.9M13.5 2v3h-3']],
    chevron: [['path', 'M4 6l4 4 4-4']],
    search: [['circle', 7, 7, 4.5], ['path', 'M10.5 10.5 14 14']],
    check: [['path', 'M3 8.5 6.5 12 13 4.5']],
    power: [['path', 'M8 1.5V8M4.3 4a5.5 5.5 0 1 0 7.4 0']],
    gauge: [['path', 'M2 11a6 6 0 0 1 12 0M8 11l3-4']],
    plus: [['path', 'M8 3v10M3 8h10']],
    copy: [['rect', 5.5, 5.5, 8, 8, 1.5], ['path', 'M10.5 5.5V3.5a1 1 0 0 0-1-1h-6a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2']],
    close: [['path', 'M4 4l8 8M12 4l-8 8']],
    help: [['circle', 8, 8, 6.5], ['path', 'M6.2 6.3a1.9 1.9 0 1 1 2.6 1.8c-.5.2-.8.6-.8 1.1v.6M8 11.6v.1']],
    'arrow-right': [['path', 'M3 8h10M9 4l4 4-4 4']],
    edit: [['path', 'M10.5 2.5l3 3L6 13H3v-3z'], ['path', 'M9 4l3 3']],
    minus: [['path', 'M3 8h10']],
    more: [['circle', 3.5, 8, 0.6], ['circle', 8, 8, 0.6], ['circle', 12.5, 8, 0.6]],
    laptop: [['rect', 3, 3.5, 10, 7, 1], ['path', 'M1.5 12.5h13']],
    server: [['rect', 2.5, 2.5, 11, 4.5, 1], ['rect', 2.5, 9, 11, 4.5, 1], ['path', 'M5 4.75h.01M5 11.25h.01']],
    sort: [['path', 'M5 13V3M2.75 5.25 5 3l2.25 2.25M11 3v10M8.75 10.75 11 13l2.25-2.25']],
    'sort-asc': [['path', 'M4 2.5v11M2 11.5l2 2 2-2M8.5 7 11 2l2.5 5M9.4 5.3h3.2M8.75 9h4.5l-4.5 4.75h4.5']],
    'sort-desc': [['path', 'M4 2.5v11M2 11.5l2 2 2-2M8.75 2h4.5l-4.5 4.75h4.5M8.5 14l2.5-5 2.5 5M9.4 12.3h3.2']],
    signal: [['path', 'M2.75 13.25v-2M6 13.25v-4.5M9.5 13.25v-7M13 13.25V2.75']]
  };

  function Icon(props) {
    var size = props.size || 16, parts = ICONS[props.name] || [];
    return h('svg', {
      className: cx('lf-icon', props.className), width: size, height: size, viewBox: '0 0 16 16',
      fill: 'none', stroke: 'currentColor', strokeWidth: 1.5, strokeLinecap: 'round', strokeLinejoin: 'round',
      'aria-hidden': props.label ? undefined : true, role: props.label ? 'img' : undefined, 'aria-label': props.label
    }, parts.map(function (p, i) {
      if (p[0] === 'path') return h('path', { key: i, d: p[1], fill: p[2] === 'fill' ? 'currentColor' : undefined, stroke: p[2] === 'fill' ? 'none' : undefined });
      if (p[0] === 'circle') return h('circle', { key: i, cx: p[1], cy: p[2], r: p[3] });
      if (p[0] === 'rect') return h('rect', { key: i, x: p[1], y: p[2], width: p[3], height: p[4], rx: p[5] });
      return h('ellipse', { key: i, cx: p[1], cy: p[2], rx: p[3], ry: p[4], transform: 'rotate(' + p[5] + ' ' + p[1] + ' ' + p[2] + ')' });
    }));
  }

  /* ---------- Button ---------- */

  function Button(props) {
    var variant = props.variant || 'secondary', size = props.size || 'md';
    var press = usePress();
    var rest = Object.assign({}, props);
    ['variant', 'size', 'icon', 'iconAfter', 'className', 'children'].forEach(function (k) { delete rest[k]; });
    return h('button', Object.assign({ type: 'button' }, rest, {
      ref: press.ref,
      className: cx('lf-btn', 'lf-btn--' + variant, 'lf-btn--' + size, !props.children && 'lf-btn--icon-only', props.className),
      onPointerDown: function (e) { press.down(); if (props.onPointerDown) props.onPointerDown(e); },
      onPointerUp: function (e) { press.up(); if (props.onPointerUp) props.onPointerUp(e); },
      onPointerLeave: function (e) { press.up(); if (props.onPointerLeave) props.onPointerLeave(e); }
    }), props.icon && h(Icon, { name: props.icon }), props.children && h('span', null, props.children), props.iconAfter && h(Icon, { name: props.iconAfter }));
  }

  /* ---------- Switch: a heavy thumb you can drag or flick ---------- */

  var TRAVEL = 16;

  function Switch(props) {
    var controlled = props.checked !== undefined;
    var st = useState(!!props.defaultChecked), inner = st[0], setInner = st[1];
    var on = controlled ? !!props.checked : inner;
    var root = useRef(null), drag = useRef(null);
    var body = useBody(on ? 1 : 0, springs.heavy, function (x) {
      var el = root.current; if (!el) return;
      el.style.setProperty('--lf-p', x.toFixed(4));
      el.querySelector('.lf-switch__thumb').style.transform = 'translateX(' + (x * TRAVEL).toFixed(2) + 'px)';
      el.dataset.lit = x > 0.5 ? 'true' : 'false';
    }, 0.0005);
    useLayoutEffect(function () { body.emit(); }, []);
    useEffect(function () { if (!drag.current) body.to(on ? 1 : 0); }, [on]);

    function commit(next) {
      body.to(next ? 1 : 0);
      if (next === on) return;
      if (!controlled) setInner(next);
      if (props.onChange) props.onChange(next);
    }
    function onDown(e) {
      if (props.disabled) return;
      e.currentTarget.setPointerCapture(e.pointerId);
      drag.current = { x0: e.clientX, p0: body.target, moved: false, lastX: e.clientX, lastT: performance.now(), vel: 0 };
      root.current.dataset.pressed = 'true';
    }
    function onMove(e) {
      var d = drag.current; if (!d) return;
      var dx = e.clientX - d.x0;
      if (Math.abs(dx) > 3) d.moved = true;
      var now = performance.now(), dt = Math.max(1, now - d.lastT) / 1000;
      d.vel = ((e.clientX - d.lastX) / TRAVEL) / dt; d.lastX = e.clientX; d.lastT = now;
      // The hand pulls the target; the heavy thumb lags behind it.
      body.to(clamp(d.p0 + dx / TRAVEL, -0.15, 1.15));
    }
    function onUp() {
      var d = drag.current; if (!d) return;
      drag.current = null; root.current.dataset.pressed = 'false';
      if (!d.moved) { commit(!on); return; }
      body.v = d.vel;
      commit(project(body.x, d.vel, springs.heavy) > 0.5);
    }

    var sw = h('button', {
      type: 'button', role: 'switch', 'aria-checked': on, 'aria-label': props.label || props['aria-label'],
      disabled: props.disabled, className: 'lf-switch', ref: root, 'data-lit': on ? 'true' : 'false',
      onPointerDown: onDown, onPointerMove: onMove, onPointerUp: onUp, onPointerCancel: onUp,
      onKeyDown: function (e) { if (e.key === ' ' || e.key === 'Enter') { e.preventDefault(); commit(!on); } }
    }, h('span', { className: 'lf-switch__fill' }), h('span', { className: 'lf-switch__thumb' }));

    if (!props.label) return sw;
    return h('div', {
      className: cx('lf-switch-row', props.disabled && 'is-disabled', props.className),
      onClick: function (e) { if (!props.disabled && !e.target.closest('.lf-switch')) commit(!on); }
    },
      props.icon && h('span', { className: 'lf-switch-row__icon' }, h(Icon, { name: props.icon })),
      h('span', { className: 'lf-switch-row__text' },
        h('span', { className: 'lf-switch-row__label' }, props.label),
        props.description && h('span', { className: 'lf-switch-row__desc' }, props.description)),
      sw);
  }

  /* ---------- SegmentedControl: the indicator glides between options ---------- */

  function SegmentedControl(props) {
    var options = props.options || [];
    var controlled = props.value !== undefined;
    var st = useState(props.defaultValue != null ? props.defaultValue : (options[0] && options[0].value));
    var value = controlled ? props.value : st[0];
    var well = useRef(null), ind = useRef(null), items = useRef([]);
    var pos = useRef({ x: 0, w: 0 });
    var bx = useBody(0, springs.glide, function (x) { pos.current.x = x; paint(); }, 0.05);
    var bw = useBody(0, springs.glide, function (w) { pos.current.w = w; paint(); }, 0.05);
    function paint() { if (ind.current) { ind.current.style.transform = 'translateX(' + pos.current.x + 'px)'; ind.current.style.width = pos.current.w + 'px'; } }
    var idx = Math.max(0, options.findIndex(function (o) { return o.value === value; }));
    var first = useRef(true);
    useLayoutEffect(function () {
      var el = items.current[idx]; if (!el) return;
      if (first.current) { bx.set(el.offsetLeft); bw.set(el.offsetWidth); first.current = false; }
      else { bx.to(el.offsetLeft); bw.to(el.offsetWidth); }
    }, [idx, options.length]);
    function choose(v) { if (props.disabled) return; if (!controlled) st[1](v); if (props.onChange) props.onChange(v); }
    function onKey(e) {
      var d = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0; if (!d) return;
      e.preventDefault();
      var n = (idx + d + options.length) % options.length;
      choose(options[n].value); if (items.current[n]) items.current[n].focus();
    }
    return h('div', { className: cx('lf-seg', props.size === 'sm' && 'lf-seg--sm', props.className), role: 'radiogroup', 'aria-label': props['aria-label'], ref: well, onKeyDown: onKey },
      h('span', { className: 'lf-seg__indicator', ref: ind, 'aria-hidden': true }),
      options.map(function (o, i) {
        var sel = i === idx;
        return h('button', {
          key: o.value, type: 'button', role: 'radio', 'aria-checked': sel, tabIndex: sel ? 0 : -1, disabled: !!props.disabled,
          className: cx('lf-seg__item', sel && 'is-selected'), ref: function (el) { items.current[i] = el; },
          onClick: function () { choose(o.value); }, 'aria-label': o.title, title: o.title
        }, o.icon && h(Icon, { name: o.icon }), o.label != null && h('span', null, o.label));
      }));
  }

  /* ---------- IgnitionButton: launch the core ---------- */

  var IGNITION_COPY = {
    idle: { title: '待命', action: '启动' },
    igniting: { title: '发射', action: '发射中' },
    orbit: { title: '在轨', action: '停止' }
  };

  // true only while the element is on screen and the window is visible; frame-costly effects mount on this
  function useOnScreen(ref, enabled) {
    var st = useState(false);
    useEffect(function () {
      var el = ref.current;
      if (!enabled || !el) { st[1](false); return; }
      var seen = true;
      function upd() { st[1](seen && document.visibilityState === 'visible'); }
      var io = typeof IntersectionObserver === 'function' ? new IntersectionObserver(function (es) { seen = es[es.length - 1].isIntersecting; upd(); }) : null;
      if (io) io.observe(el);
      document.addEventListener('visibilitychange', upd); upd();
      return function () { if (io) io.disconnect(); document.removeEventListener('visibilitychange', upd); };
    }, [enabled]);
    return st[0];
  }

  /* How far the cap sits in its seat: 0 is up, 1 is bottomed out. The state decides where it rests, so height shows it. */
  var IGNITION_REST = { idle: 0, igniting: 1, orbit: 0.6 };
  var IGNITION_THUNK = { mass: 1, stiffness: 1400, damping: 75 };
  var IGNITION_RETURN = { mass: 1, stiffness: 420, damping: 14 };
  function useKeycap(state) {
    var el = useRef(null), rest = IGNITION_REST[state] || 0;
    var body = useBody(rest, IGNITION_RETURN, function (x) {
      if (el.current) el.current.style.setProperty('--lf-press', x.toFixed(4));
    }, 0.001);
    React.useLayoutEffect(function () { body.emit(); }, []);
    useEffect(function () { body.to(rest, { spring: rest > body.x ? IGNITION_THUNK : IGNITION_RETURN }); }, [rest]);
    return {
      ref: el,
      down: function () { body.to(1, { spring: IGNITION_THUNK }); },
      up: function () { body.to(rest, { spring: IGNITION_RETURN }); }
    };
  }

  function IgnitionButton(props) {
    var state = props.state || 'idle';
    var root = useRef(null);
    // motion (exhaust while launching, the rocket circling in orbit) only exists while someone can see it
    var visible = useOnScreen(root, !reducedMotion());
    var seen = visible && state !== 'idle', calling = visible && state === 'idle';
    var burn = seen && state === 'igniting', circling = seen && state === 'orbit';
    var copy = Object.assign({}, IGNITION_COPY[state], (props.copy || {})[state]);
    var press = useKeycap(state);
    var st = useState(0), now = st[0], setNow = st[1];
    useEffect(function () {
      if (state !== 'orbit' || !props.since) return;
      var t = setInterval(function () { setNow(Date.now()); }, 1000); setNow(Date.now());
      return function () { clearInterval(t); };
    }, [state, props.since]);
    var elapsed = props.since && now ? (now - props.since) / 1000 : 0;
    return h('div', { className: cx('lf-ignite', 'is-' + state, seen && 'is-live', calling && 'is-calling', props.className), ref: root },
      h('div', { className: 'lf-ignite__seat' }, h('button', {
        type: 'button', ref: press.ref, className: 'lf-ignite__core', disabled: state === 'igniting',
        'aria-label': copy.action + (props.subtitle ? ' — ' + props.subtitle : ''),
        onPointerDown: press.down, onPointerUp: press.up, onPointerLeave: press.up, onPointerCancel: press.up, onBlur: press.up,
        onKeyDown: function (e) { if ((e.key === ' ' || e.key === 'Enter') && !e.repeat) press.down(); },
        onKeyUp: function (e) { if (e.key === ' ' || e.key === 'Enter') press.up(); },
        onClick: function () { if (props.onToggle) props.onToggle(state === 'orbit' ? 'land' : 'launch'); }
      },
        // waiting: two rings fall inward and land on the button; gone as soon as someone reaches for it
        calling && h('span', { className: 'lf-ignite__call', 'aria-hidden': true }), calling && h('span', { className: 'lf-ignite__call is-late', 'aria-hidden': true }),
        h('span', { className: 'lf-ignite__track', 'aria-hidden': true },
          circling
            ? h('span', { className: 'lf-ignite__orbiter' }, h('span', { className: 'lf-ignite__trail' }), h('span', { className: 'lf-ignite__sat' }, h(Icon, { name: 'rocket', size: 14 })))
            : h('span', { className: 'lf-ignite__beacon' })),
        state === 'orbit' ? h(Icon, { name: 'power', size: 26 }) : h('span', { className: cx('lf-ignite__craft', burn && 'is-burning'), 'aria-hidden': true },
          h(Icon, { name: 'rocket', size: 28 }),
          burn && h('span', { className: 'lf-ignite__exhaust' },
            h('span', { className: 'lf-ignite__flame' }), h('span', { className: 'lf-ignite__flame is-inner' }),
            [0, 1, 2, 3, 4].map(function (i) { return h('span', { key: i, className: 'lf-ignite__spark', style: { '--i': i } }); }))),
        h('span', { className: 'lf-ignite__action' }, copy.action))),
      h('div', { className: 'lf-ignite__meta' },
        h('div', { className: 'lf-ignite__title' }, copy.title),
        h('div', { className: 'lf-ignite__sub' },
          state === 'orbit' && h('span', { className: 'lf-beacon', 'aria-hidden': true }),
          state === 'orbit' && props.since ? fmt.clock(elapsed) : null,
          state === 'orbit' && props.since && props.subtitle ? ' · ' : null,
          props.subtitle),
        props.error && h('div', { className: 'lf-ignite__error', role: 'alert' }, h(Icon, { name: 'alert-circle' }), props.error)));
  }

  /* ---------- TelemetryStat: live speed on a log-scale thrust dial ---------- */

  var DIAL = { cx: 110, cy: 104, r: 78, start: 150, sweep: 240, decades: 6 };
  function dialPoint(t, r) {
    var a = (DIAL.start + DIAL.sweep * t) * Math.PI / 180;
    return { x: DIAL.cx + Math.cos(a) * r, y: DIAL.cy + Math.sin(a) * r };
  }
  function dialSpoke(t, inner, outer) {
    var a = dialPoint(t, inner), b = dialPoint(t, outer);
    return 'M' + a.x.toFixed(2) + ' ' + a.y.toFixed(2) + 'L' + b.x.toFixed(2) + ' ' + b.y.toFixed(2);
  }
  var DIAL_ARC = (function () {
    var a = dialPoint(0, DIAL.r), b = dialPoint(1, DIAL.r);
    return 'M' + a.x.toFixed(2) + ' ' + a.y.toFixed(2) + 'A' + DIAL.r + ' ' + DIAL.r + ' 0 1 1 ' + b.x.toFixed(2) + ' ' + b.y.toFixed(2);
  })();
  var DIAL_MAJOR = '', DIAL_MINOR = '';
  for (var dialD = 0; dialD <= DIAL.decades; dialD++) {
    DIAL_MAJOR += dialSpoke(dialD / DIAL.decades, DIAL.r + 4, DIAL.r + 12);
    if (dialD < DIAL.decades) for (var dialM = 2; dialM < 10; dialM++) DIAL_MINOR += dialSpoke((dialD + Math.log10(dialM)) / DIAL.decades, DIAL.r + 4, DIAL.r + 8);
  }
  var DIAL_MARKS = [[0, '100B'], [2, '10K'], [4, '1M'], [6, '100M']].map(function (m) {
    var p = dialPoint(m[0] / DIAL.decades, DIAL.r - 17);
    return { key: m[1], x: p.x, y: p.y };
  });
  /* 100 B/s sits at the start of the arc, 100 MB/s at the end. */
  function dialLevel(bps) {
    return Math.max(0, Math.min(1, (Math.log10(Math.max(bps || 0, 100)) - 2) / DIAL.decades));
  }

  function TelemetryStat(props) {
    var s = fmt.speed(props.value);
    var name = props.label || (props.direction === 'up' ? '上行' : '下行');
    var live = props.value != null && !isNaN(props.value);
    var peak = live ? Math.max.apply(null, (props.history || []).concat([props.value])) : 0;
    return h('div', { className: cx('lf-telemetry', props.className) },
      h('svg', { className: 'lf-telemetry__dial', viewBox: '0 0 220 186', role: 'img', 'aria-label': name + ' ' + (live ? s.value + ' ' + s.unit : '无数据') },
        h('path', { className: 'lf-telemetry__minor', d: DIAL_MINOR }),
        h('path', { className: 'lf-telemetry__major', d: DIAL_MAJOR }),
        h('path', { className: 'lf-telemetry__track', d: DIAL_ARC }),
        live && h('path', { className: 'lf-telemetry__level', d: DIAL_ARC, pathLength: 1, strokeDasharray: dialLevel(props.value) + ' 1' }),
        live && peak >= 100 && h('path', { className: 'lf-telemetry__peak', d: dialSpoke(dialLevel(peak), DIAL.r - 6, DIAL.r + 6) }),
        DIAL_MARKS.map(function (m) { return h('text', { key: m.key, className: 'lf-telemetry__mark', x: m.x, y: m.y }, m.key); }),
        h('text', { className: 'lf-telemetry__label', x: DIAL.cx, y: DIAL.cy - 36 }, (props.direction === 'up' ? '↑ ' : '↓ ') + name),
        h('text', { className: 'lf-telemetry__num', x: DIAL.cx, y: DIAL.cy }, s.value),
        s.unit && h('text', { className: 'lf-telemetry__unit', x: DIAL.cx, y: DIAL.cy + 30 }, s.unit)),
      props.total != null && h('div', { className: 'lf-telemetry__total' }, '累计 ' + fmt.bytes(props.total)));
  }

  /* ---------- TelemetryHistory: the last 24 seconds as two waves mirrored on one axis ---------- */

  var TRACE = { samples: 24, w: 480, h: 96 };
  var TRACE_GRID = (function () {
    var d = '', i;
    for (i = 1; i < 12; i++) d += 'M' + i * TRACE.w / 12 + ' 0V' + TRACE.h;
    for (i = 1; i < 6; i++) if (i !== 3) d += 'M0 ' + i * TRACE.h / 6 + 'H' + TRACE.w;
    return d;
  })();
  /* Both waves share one linear scale: the busiest sample, rounded up to 1, 2 or 5 of its unit. */
  function traceScale(samples) {
    var max = Math.max.apply(null, samples.concat([1024]));
    var unit = Math.pow(1024, Math.floor(Math.log(max) / Math.log(1024))), m = max / unit;
    var step = Math.pow(10, Math.floor(Math.log10(m))), n = m / step;
    return (n <= 1 ? 1 : n <= 2 ? 2 : n <= 5 ? 5 : 10) * step * unit;
  }
  /* side 1 rises from the axis, side -1 falls; newest sample is always at the right edge. */
  function traceWave(samples, side, scale) {
    var offset = TRACE.samples - samples.length;
    return samples.map(function (v, i) {
      var x = (offset + i) / (TRACE.samples - 1) * TRACE.w;
      return x.toFixed(1) + ',' + (TRACE.h / 2 - side * (v / scale) * (TRACE.h / 2 - 2)).toFixed(1);
    }).join(' ');
  }
  /* The closed shape under a wave. Samples are one second apart, so its area is the bytes moved. */
  function traceArea(samples, side, scale) {
    var firstX = (TRACE.samples - samples.length) / (TRACE.samples - 1) * TRACE.w;
    return firstX.toFixed(1) + ',' + TRACE.h / 2 + ' ' + traceWave(samples, side, scale) + ' ' + TRACE.w + ',' + TRACE.h / 2;
  }
  /* One row of cells in the legend grid, so peak and volume line up across the two directions. */
  function traceRow(side, name, samples) {
    var has = samples.length > 1, peak = has ? fmt.speed(Math.max.apply(null, samples)) : null;
    return [
      h('span', { key: side + 's', className: 'lf-trace__swatch is-' + side }),
      h('span', { key: side + 'n' }, name),
      h('span', { key: side + 'p' }, has ? '峰值' : ''),
      h('span', { key: side + 'v', className: 'is-num' }, has ? peak.value + ' ' + peak.unit : ''),
      h('span', { key: side + 'b', className: 'is-num' }, has ? fmt.bytes(samples.reduce(function (a, v) { return a + v; }, 0)) : '')
    ];
  }
  function TelemetryHistory(props) {
    var down = (props.down || []).slice(-TRACE.samples), up = (props.up || []).slice(-TRACE.samples);
    var scale = traceScale(down.concat(up)), full = fmt.speed(scale);
    return h('div', { className: cx('lf-trace', props.className) },
      h('div', { className: 'lf-trace__head' },
        h('span', null, '近 24 秒'),
        h('div', { className: 'lf-trace__legend' }, traceRow('down', '下行', down), traceRow('up', '上行', up))),
      h('svg', { className: 'lf-trace__plot', viewBox: '0 0 ' + TRACE.w + ' ' + TRACE.h, preserveAspectRatio: 'none', 'aria-hidden': true },
        h('path', { className: 'lf-trace__grid', d: TRACE_GRID, vectorEffect: 'non-scaling-stroke' }),
        h('path', { className: 'lf-trace__axis-line', d: 'M0 ' + TRACE.h / 2 + 'H' + TRACE.w, vectorEffect: 'non-scaling-stroke' }),
        up.length > 1 && h('polygon', { className: 'lf-trace__area is-up', points: traceArea(up, -1, scale) }),
        down.length > 1 && h('polygon', { className: 'lf-trace__area is-down', points: traceArea(down, 1, scale) }),
        up.length > 1 && h('polyline', { className: 'lf-trace__wave is-up', points: traceWave(up, -1, scale), vectorEffect: 'non-scaling-stroke' }),
        down.length > 1 && h('polyline', { className: 'lf-trace__wave is-down', points: traceWave(down, 1, scale), vectorEffect: 'non-scaling-stroke' })),
      h('div', { className: 'lf-trace__axis' }, h('span', null, 'T−24S'), down.length > 1 && h('span', null, '满量程 ' + full.value + ' ' + full.unit), h('span', null, 'T−0')));
  }

  /* ---------- LatencyBadge: signal bars + number, no traffic-light colours ---------- */

  function bars(ms) {
    if (ms == null || ms === 'timeout' || ms <= 0) return 0;
    return ms < 100 ? 4 : ms < 200 ? 3 : ms < 400 ? 2 : 1;
  }

  function LatencyBadge(props) {
    var ms = props.ms, timeout = ms === 'timeout' || ms === 0, n = bars(ms);
    // Untested reads as nothing at all: the group's 测速 button is the call to action, not a placeholder dash.
    if (ms == null && !props.testing) return h('span', { className: cx('lf-latency', 'is-untested', props.className), title: '未测速' });
    var text = props.testing ? '···' : timeout ? '超时' : ms + ' ms';
    return h('span', {
      className: cx('lf-latency', 'lf-latency--q' + n, timeout && !props.testing && 'is-timeout', props.testing && 'is-testing', props.className),
      title: timeout ? '失联' : undefined
    },
      h('span', { className: 'lf-latency__bars', 'aria-hidden': true }, [0, 1, 2, 3].map(function (i) {
        return h('span', { key: i, className: cx('lf-latency__bar', i < n && 'is-on'), style: { height: (4 + i * 3) + 'px', animationDelay: (i * 90) + 'ms' } });
      })),
      h('span', { className: 'lf-latency__text' }, text));
  }

  /* ---------- Flags: country / region from a node's name ---------- */

  // [code, names (any case), extra uppercase codes]. Codes match only in capitals, next to non-letters ("HK01", "JP-Tokyo").
  var FLAG_RULES = [
    ['HK', /香港|港[0-9 ]|Hong ?Kong/i, 'HKG'], ['MO', /澳门|澳門|Maca[uo]/i, 'MAC'], ['TW', /台湾|台灣|台北|Taiwan/i, 'TWN'],
    ['JP', /日本|东京|東京|大阪|Japan|Tokyo|Osaka/i, 'JPN'], ['KR', /韩国|韓國|首尔|Korea|Seoul/i, 'KOR'],
    ['SG', /新加坡|狮城|Singapore/i, 'SGP'], ['US', /美国|美國|洛杉矶|圣何塞|西雅图|纽约|硅谷|芝加哥|达拉斯|United States|Los Angeles|San Jose|Seattle|New York|Chicago|Dallas/i, 'USA'],
    ['GB', /英国|英國|伦敦|United Kingdom|London/i, 'UK'], ['DE', /德国|德國|法兰克福|Germany|Frankfurt/i, 'DEU'],
    ['FR', /法国|法國|巴黎|France|Paris/i, 'FRA'], ['NL', /荷兰|荷蘭|阿姆斯特丹|Netherlands|Amsterdam/i, 'NLD'],
    ['CA', /加拿大|多伦多|Canada|Toronto/i, 'CAN'], ['AU', /澳大利亚|澳洲|悉尼|Australia|Sydney/i, 'AUS'],
    ['RU', /俄罗斯|莫斯科|Russia|Moscow/i, 'RUS'], ['IN', /印度(?!尼)|孟买|India|Mumbai/i, 'IND'], ['TR', /土耳其|Turkey|Türkiye/i, 'TUR'],
    ['TH', /泰国|曼谷|Thailand|Bangkok/i, 'THA'], ['VN', /越南|Vietnam/i, 'VNM'], ['MY', /马来西亚|吉隆坡|Malaysia/i, 'MYS'],
    ['PH', /菲律宾|Philippines/i, 'PHL'], ['ID', /印尼|印度尼西亚|雅加达|Indonesia|Jakarta/i, 'IDN'],
    ['AR', /阿根廷|Argentina/i, 'ARG'], ['BR', /巴西|Brazil/i, 'BRA'], ['AE', /阿联酋|迪拜|Dubai|Emirates/i, 'UAE'],
    ['CN', /中国|中國|回国|China/i, 'CHN']
  ];
  FLAG_RULES.forEach(function (r) { r.push(new RegExp('(^|[^A-Za-z])(' + r[0] + '|' + r[2] + ')(?![A-Za-z])')); });

  // "US" → 🇺🇸; anything but a two-letter code → ''.
  function emojiOfCode(code) {
    if (!/^[A-Za-z]{2}$/.test(code || '')) return '';
    var upper = code.toUpperCase();
    return String.fromCodePoint(0x1F1E6 + upper.charCodeAt(0) - 65, 0x1F1E6 + upper.charCodeAt(1) - 65);
  }

  // Returns {code, emoji} or null. A name that already carries a flag keeps its own.
  function flagOf(name) {
    if (!name || /[\uD83C][\uDDE6-\uDDFF][\uD83C][\uDDE6-\uDDFF]/.test(name)) return null;
    // CN2 names a paid line, which carries traffic to any country; it says nothing about China.
    var place = name.replace(/(^|[^A-Za-z])CN2(?![0-9])/g, '$1');
    for (var i = 0; i < FLAG_RULES.length; i++) {
      if (FLAG_RULES[i][1].test(place) || FLAG_RULES[i][3].test(place)) {
        var code = FLAG_RULES[i][0];
        return { code: code, emoji: emojiOfCode(code) };
      }
    }
    return null;
  }

  function Flag(props) {
    var f = flagOf(props.name);
    if (!f) return props.placeholder ? h('span', { className: 'lf-flag lf-flag--none', 'aria-hidden': true }, h(Icon, { name: 'globe', size: 12 })) : null;
    return h('span', { className: 'lf-flag', role: 'img', 'aria-label': f.code, title: f.code }, f.emoji);
  }

  /* ---------- ProxyNode ---------- */

  function ProxyNode(props) {
    var press = usePress();
    return h('button', {
      type: 'button', role: 'radio', 'aria-checked': !!props.selected, ref: press.ref,
      disabled: props.disabled, 'aria-disabled': props.readOnly || undefined,
      className: cx('lf-node', props.selected && 'is-selected', props.readOnly && 'is-readonly', props.className),
      onPointerDown: props.readOnly ? undefined : press.down, onPointerUp: press.up, onPointerLeave: press.up,
      onClick: function () { if (!props.readOnly && props.onSelect) props.onSelect(props.name); }
    },
      h('span', { className: 'lf-node__top' },
        h('span', { className: 'lf-node__name' }, h(Flag, { name: props.name }), props.name),
        props.selected && h('span', { className: 'lf-node__selected-label' }, '已选'),
        h('span', { className: 'lf-node__dot', 'aria-hidden': true })),
      h('span', { className: 'lf-node__bottom' },
        h('span', { className: 'lf-node__type' }, props.type, props.udp ? ' · UDP' : ''),
        h(LatencyBadge, { ms: props.delay, testing: props.testing })));
  }

  /* ---------- ProxyGroup: collapsible group of nodes ---------- */

  /* ---------- Name sort: one A→Z for Chinese and English names ---------- */

  // Han names file under their pinyin initial, so 东京 sits among the D's next to Dallas,
  // not after every Latin name. Within a letter the zh pinyin collator decides, with
  // numbers compared as numbers (节点 2 before 节点 10). Leading flags and symbols are skipped.
  var nameCollator = typeof Intl !== 'undefined' && Intl.Collator ? new Intl.Collator('zh-Hans-CN-u-co-pinyin', { numeric: true, sensitivity: 'base' }) : null;
  // the first character in pinyin order of each initial: 阿 is the first A, 八 the first B…
  var PINYIN_BOUNDS = '阿八嚓哒妸发旮哈讥讥咔垃妈拏噢妑七呥仨他屲屲屲夕丫帀';
  function nameKey(name) { return String(name || '').replace(/^[^\p{L}\p{N}]+/u, ''); }
  function nameInitial(name) {
    var c = nameKey(name).normalize('NFD').charAt(0);
    if (!c) return '';
    if (/[0-9]/.test(c)) return '0' + c;
    if (/[a-z]/i.test(c)) return 'a' + c.toLowerCase();
    if (/[一-鿿]/.test(c) && nameCollator) {
      var letter = '';
      for (var i = 0; i < 26; i++) {
        if (PINYIN_BOUNDS[i] === PINYIN_BOUNDS[i + 1]) continue; // no Han syllable starts with i, u, v: the shared bound belongs to the next letter
        if (nameCollator.compare(c, PINYIN_BOUNDS[i]) >= 0) letter = String.fromCharCode(97 + i); else break;
      }
      return 'a' + (letter || 'a');
    }
    return 'z' + c;
  }
  function compareNames(a, b) {
    var ia = nameInitial(a), ib = nameInitial(b);
    if (ia !== ib) return ia < ib ? -1 : 1;
    var ka = nameKey(a), kb = nameKey(b);
    return nameCollator ? nameCollator.compare(ka, kb) : ka < kb ? -1 : ka > kb ? 1 : 0;
  }
  // sort: 'none' keeps the config's order, 'asc' A→Z, 'desc' Z→A.
  function sortByName(list, sort, nameOf) {
    if (!sort || sort === 'none') return list;
    var get = nameOf || function (n) { return n.name; }, sign = sort === 'desc' ? -1 : 1;
    return list.slice().sort(function (a, b) { return sign * compareNames(get(a), get(b)); });
  }
  // sortNodes adds 'latency' on top: fastest first, then timeouts, then untested, each by name.
  function latencyRank(delay) {
    if (delay == null) return 2;
    return delay === 'timeout' || delay === 0 ? 1 : 0;
  }
  function sortNodes(list, sort) {
    if (sort !== 'latency') return sortByName(list, sort);
    return list.slice().sort(function (a, b) {
      var ra = latencyRank(a.delay), rb = latencyRank(b.delay);
      if (ra !== rb) return ra - rb;
      return (ra === 0 ? a.delay - b.delay : 0) || compareNames(a.name, b.name);
    });
  }
  var SORT_LABEL = { none: '默认顺序', asc: '名称 A→Z', desc: '名称 Z→A', latency: '延迟 低→高' };
  var SORT_ICON = { none: 'sort', asc: 'sort-asc', desc: 'sort-desc', latency: 'signal' };
  // Controlled with sort + onChange; without them it keeps its own state.
  function useNameSort(props) {
    var st = useState(props.defaultSort || 'none');
    var value = props.sort || st[0];
    return [value, function (next) { if (!props.sort) st[1](next); if (props.onSortChange) props.onSortChange(next); }];
  }
  function SortMenu(props) {
    var v = SORT_LABEL[props.value] ? props.value : 'none';
    var menu = useState(null), button = useRef(null), shutAt = useRef(0);
    return h(React.Fragment, null,
      h('button', { type: 'button', ref: button, className: cx('lf-sort', v !== 'none' && 'is-on', props.className),
        'aria-haspopup': 'menu', 'aria-expanded': !!menu[0], 'aria-label': '节点排序：' + SORT_LABEL[v], title: '节点排序：' + SORT_LABEL[v],
        onClick: function () {
          // the press that closes the menu also lands here; don't reopen it
          if (Date.now() - shutAt.current < 400) return;
          var r = button.current.getBoundingClientRect();
          menu[1]({ x: r.left, y: r.bottom + 4 });
        } },
        h(Icon, { name: SORT_ICON[v], size: 14 })),
      h(ContextMenu, { open: !!menu[0], x: menu[0] ? menu[0].x : 0, y: menu[0] ? menu[0].y : 0, label: '节点排序',
        onClose: function () { shutAt.current = Date.now(); menu[1](null); if (button.current) button.current.focus({ preventScroll: true }); },
        items: ['none', 'asc', 'desc', 'latency'].map(function (k) {
          return { label: SORT_LABEL[k], icon: SORT_ICON[k], selected: k === v, onSelect: function () { props.onChange(k); } };
        }) }));
  }

  function ProxyGroup(props) {
    var ns = useNameSort(props), nodes = sortNodes(props.nodes || [], ns[0]);
    var st = useState(props.defaultOpen !== false), open = st[0], setOpen = st[1];
    var wrap = useRef(null), inner = useRef(null), chev = useRef(null);
    var hb = useBody(open ? 1 : 0, springs.glide, function (p) {
      if (!wrap.current || !inner.current) return;
      var full = inner.current.offsetHeight;
      wrap.current.style.height = p >= 1 ? 'auto' : Math.max(0, p * full) + 'px';
      wrap.current.style.opacity = clamp(p * 1.4, 0, 1);
      if (chev.current) chev.current.style.transform = 'rotate(' + ((1 - p) * -90) + 'deg)';
    }, 0.002);
    useLayoutEffect(function () { hb.emit(); }, []);
    function toggle() { var n = !open; setOpen(n); hb.to(n ? 1 : 0); }
    var selectable = !props.type || props.type === 'Selector';
    return h('section', { className: cx('lf-group', props.className) },
      h('header', { className: 'lf-group__head' },
        h('button', { type: 'button', className: 'lf-group__toggle', 'aria-expanded': open, onClick: toggle },
          h('span', { ref: chev, className: 'lf-group__chev' }, h(Icon, { name: 'chevron' })),
          h('span', { className: 'lf-group__name' }, props.name),
          h('span', { className: 'lf-group__type' }, props.type || 'Selector'),
          props.meta && h('span', { className: 'lf-group__meta' }, props.meta),
          props.activity > 0 && h('span', { className: 'lf-group__activity' }, props.activity + ' 条连接'),
          // label: what the readout means; via: groups passed on the way; leaf: where traffic finally goes
          (props.leaf != null || props.now) && h('span', { className: 'lf-group__now' },
            h('span', { className: 'lf-group__label' }, props.label || (props.type && props.type !== 'Selector' ? '自动选中' : '当前节点')),
            props.via && h('span', { className: 'lf-group__via' }, props.via),
            h('span', { className: 'lf-group__leaf' }, h(Flag, { name: props.leaf != null ? props.leaf : props.now }), props.leaf != null ? props.leaf : props.now)),
          h('span', { className: 'lf-group__count' }, nodes.length + ' 个')),
        nodes.length > 1 && h(SortMenu, { value: ns[0], onChange: ns[1], className: 'lf-group__sort' }),
        h(Button, { variant: 'ghost', size: 'sm', icon: 'refresh', className: cx('lf-group__test', props.testing && 'is-spinning'), 'aria-label': '测速：' + props.name, onClick: props.onTest, disabled: props.testing || props.testDisabled }, props.testing ? '测速中' : '测速')),
      h('div', { className: 'lf-group__body', ref: wrap },
        h('div', { className: 'lf-group__grid', ref: inner, role: 'radiogroup', 'aria-label': props.name },
          nodes.map(function (n) {
            return h(ProxyNode, Object.assign({ key: n.name }, n, {
              selected: n.name === props.now, readOnly: props.readOnly || !selectable, testing: props.testing,
              onSelect: function (name) { if (props.onSelect) props.onSelect(name); }
            }));
          }))));
  }

  /* ---------- Trajectory: routes from this Mac to exit nodes over a dot-matrix world map ---------- */

  // Natural Earth 110m land, rasterized to 3° cells (120 × 46, 81°N → 57°S); one hex row per latitude band, high bit = 180°W.
  var WORLD = { step: 3, top: 81, cols: 120, rows: 46, land: [
    '0000005ffffffc01f808003e000000',
    '00001fbfcffffc004001e03fc03000',
    '00003fbfc07ff800000711fffe3000',
    '83f8f7dffc3ff8003e06bfffffffe6',
    'efffffffbe3fe000ffffffffffffff',
    '6fffffffbe3e0e01ffffffffffffff',
    '0ffffff83c1c0007dfffffffffffff',
    '03c7fff83e000023dfffffffffffc0',
    '0200ffffff8000739fffffffffc0c0',
    '00007fffffc0007ffffffffffff080',
    '00007fffffc0003ffffffffffff000',
    '00003fffff00001fffffffffffd000',
    '00003ffffa00007ffc77ffffff9800',
    '00003ffff000007bfffffffffe3000',
    '00001fffe000007f97fffffffa6000',
    '00000fffe000007f00fffffff3e000',
    '000007ff800000fffffffffff90000',
    '000003f0800000fffff7fffff80000',
    '000003f0400001fffffbfffff80000',
    '000001f0c00003ffff7f1fffe80000',
    '00800076380003ffffff0f9f800000',
    '0000003f000003ffffbc0f1f080000',
    '00000007000003fffff0060f8c0000',
    '000000015f0001fffff80603140000',
    '00000000ff8001fffff803040c0000',
    '000000003fe00073fff0000e300000',
    '000000007ff00001ffe00006fe0000',
    '000000007ff80001ffc00006f9e000',
    '000000007fff0000ff80000329fa00',
    '000000007fff8000ff800001f03880',
    '000000003fff0000ffc00000041640',
    '000000003ffe0000ffc8000003d000',
    '000000001ffe0000fff8000007d801',
    '000000000ffe0000ff1800001ffc00',
    '000000000ffc0000ff3000003ffe00',
    '000000000ff000007f1000003ffe00',
    '000000000ff000007e0000003ffe00',
    '000000000fe000003c0000003ffe00',
    '000000000fc0000038000000387e00',
    '000000001f80000000000000003c02',
    '000000001f00000000000000000806',
    '000000001e0000000000000000080c',
    '000000001c00000000000000000008',
    '000000001c00000000001000000000',
    '000000001880000000000000000000',
    '000000000c00000000000000000000'] };
  var LAND_BITS = WORLD.land.map(function (row) {
    return row.split('').map(function (c) { return ('000' + parseInt(c, 16).toString(2)).slice(-4); }).join('').slice(-WORLD.cols);
  });
  // where a node is, by city name first, then by the flag its name resolves to
  var CITY_GEO = [
    [/洛杉矶|Los ?Angeles|\bLAX?\b/i, -118.2, 34.1], [/圣何塞|硅谷|San ?Jose|\bSJC\b/i, -121.9, 37.3], [/西雅图|Seattle/i, -122.3, 47.6],
    [/纽约|New ?York|\bNYC?\b/i, -74, 40.7], [/芝加哥|Chicago/i, -87.6, 41.9], [/达拉斯|Dallas/i, -96.8, 32.8], [/迈阿密|Miami/i, -80.2, 25.8],
    [/大阪|Osaka/i, 135.5, 34.7], [/伦敦|London/i, -0.1, 51.5], [/法兰克福|Frankfurt/i, 8.7, 50.1], [/阿姆斯特丹|Amsterdam/i, 4.9, 52.4],
    [/悉尼|Sydney/i, 151.2, -33.9], [/墨尔本|Melbourne/i, 145, -37.8], [/多伦多|Toronto/i, -79.4, 43.7], [/温哥华|Vancouver/i, -123.1, 49.3]
  ];
  var COUNTRY_GEO = {
    CN: [121.5, 31.2], HK: [114.2, 22.3], MO: [113.5, 22.2], TW: [121.5, 25], JP: [139.7, 35.7], KR: [127, 37.6], SG: [103.8, 1.35],
    US: [-118.2, 34.1], CA: [-79.4, 43.7], GB: [-0.1, 51.5], DE: [8.7, 50.1], FR: [2.35, 48.85], NL: [4.9, 52.4], RU: [37.6, 55.75],
    IN: [72.9, 19.1], TR: [29, 41], TH: [100.5, 13.75], VN: [105.85, 21], MY: [101.7, 3.1], PH: [121, 14.6], ID: [106.8, -6.2],
    AU: [151.2, -33.9], AR: [-58.4, -34.6], BR: [-46.6, -23.5], AE: [55.3, 25.2]
  };
  function geoOf(name) {
    for (var i = 0; i < CITY_GEO.length; i++) if (CITY_GEO[i][0].test(name || '')) return [CITY_GEO[i][1], CITY_GEO[i][2]];
    var f = flagOf(name);
    return f && COUNTRY_GEO[f.code] ? COUNTRY_GEO[f.code] : null;
  }

  // the land never changes with zoom or traffic: render it once and move it as one group
  var LandDots = React.memo(function (p) {
    var out = [], shift = Math.round((p.lon0 + 180) / WORLD.step);
    for (var r = 0; r < WORLD.rows; r++) for (var c = 0; c < WORLD.cols; c++) {
      if (LAND_BITS[r][(c + shift + WORLD.cols) % WORLD.cols] !== '1') continue;
      out.push(h('circle', { key: r + '-' + c, cx: (p.left + (c + 0.5) * p.cell).toFixed(1), cy: (p.top + (r + 0.5) * p.cell).toFixed(1), r: (p.cell * 0.26).toFixed(2), className: 'lf-traj__land' }));
    }
    return out;
  });

  var HOME = { k: 1, x: 0, y: 0 };

  function Trajectory(props) {
    // Draw in the box's own pixels so nodes and labels keep their size at any
    // width; only the map grows. An explicit width keeps the fixed 640 × 260 box.
    var fig = useRef(null), ms = useState(0), measured = ms[0], setMeasured = ms[1];
    useLayoutEffect(function () {
      if (props.width || !fig.current || typeof ResizeObserver === 'undefined') return;
      var ro = new ResizeObserver(function (entries) {
        var w = Math.round(entries[0].contentRect.width);
        if (w > 0) setMeasured(w);
      });
      ro.observe(fig.current);
      return function () { ro.disconnect(); };
    }, [props.width]);
    var W = props.width || measured || 640, H = props.height || Math.round(W * 260 / 640), still = reducedMotion();
    var nodes = props.nodes || [], zoomable = props.zoomable !== false, maxK = props.maxZoom || 5;
    // the map runs 360° starting at centerLon − 180; 150°E keeps Europe left, Asia centre, the Americas right
    var lon0 = (props.centerLon != null ? props.centerLon : 150) - 180;
    var cell = Math.min(W / WORLD.cols, H / WORLD.rows), mw = cell * WORLD.cols, mh = cell * WORLD.rows;
    var left = (W - mw) / 2, topY = (H - mh) / 2;
    function at(lon, lat) {
      var u = (((lon - lon0) % 360) + 360) % 360;
      return [left + u / WORLD.step * cell, topY + (WORLD.top - lat) / WORLD.step * cell];
    }

    // view = screen = map × k + (x, y); clamped so the map always covers the box
    var vs = useState(HOME), view = vs[0], setView = vs[1];
    var svg = useRef(null), live = useRef(HOME), drag = useRef(null), from = useRef(HOME), goal = useRef(HOME);
    function clampV(v) {
      var k = clamp(v.k, 1, maxK);
      return { k: k, x: clamp(v.x, W - W * k, 0), y: clamp(v.y, H - H * k, 0) };
    }
    function show(v) { live.current = v; setView(v); }
    var tween = useBody(1, springs.glide, function (p) {
      var a = from.current, b = goal.current;
      show({ k: a.k + (b.k - a.k) * p, x: a.x + (b.x - a.x) * p, y: a.y + (b.y - a.y) * p });
    }, 0.001);
    function glideTo(v) { from.current = live.current; goal.current = clampV(v); tween.set(0); tween.to(1); }
    // zoom by factor f keeping the map point under (px, py) in place
    function zoomAt(f, px, py, animate) {
      var v = live.current, k = clamp(v.k * f, 1, maxK), s = k / v.k;
      var next = clampV({ k: k, x: px - (px - v.x) * s, y: py - (py - v.y) * s });
      if (animate) glideTo(next); else { tween.stop(); show(next); }
    }
    function local(e) {
      var r = svg.current.getBoundingClientRect();
      return [(e.clientX - r.left) * W / r.width, (e.clientY - r.top) * H / r.height];
    }
    // trackpad pinch arrives as ctrl+wheel; ⌘/Ctrl + wheel zooms too; a plain wheel keeps scrolling the page
    useEffect(function () {
      var el = svg.current; if (!el || !zoomable) return;
      function wheel(e) {
        if (!e.ctrlKey && !e.metaKey) return;
        e.preventDefault();
        var p = local(e); zoomAt(Math.exp(-e.deltaY * (e.ctrlKey ? 0.01 : 0.002)), p[0], p[1], false);
      }
      el.addEventListener('wheel', wheel, { passive: false });
      return function () { el.removeEventListener('wheel', wheel); };
    }, [zoomable, W, H, maxK]);
    useEffect(function () { show(clampV(live.current)); }, [W, H]);

    var handlers = zoomable ? {
      onPointerDown: function (e) {
        if (live.current.k <= 1 || e.button !== 0) return;
        tween.stop(); svg.current.setPointerCapture(e.pointerId);
        var p = local(e); drag.current = { px: p[0], py: p[1], x: live.current.x, y: live.current.y };
      },
      onPointerMove: function (e) {
        var d = drag.current; if (!d) return;
        var p = local(e);
        show(clampV({ k: live.current.k, x: d.x + p[0] - d.px, y: d.y + p[1] - d.py }));
      },
      onPointerUp: function () { drag.current = null; },
      onPointerCancel: function () { drag.current = null; },
      onDoubleClick: function (e) { var p = local(e); zoomAt(e.shiftKey ? 0.5 : 2, p[0], p[1], true); }
    } : {};

    var k = view.k;
    function scr(p) { return [p[0] * k + view.x, p[1] * k + view.y]; }
    var o = props.originGeo === null ? null : scr(at.apply(null, props.originGeo || COUNTRY_GEO.CN));
    var zoomed = k > 1.001;
    return h('figure', { ref: fig, className: cx('lf-traj', zoomable && 'is-zoomable', zoomed && 'is-zoomed', props.className) },
      h('svg', Object.assign({ ref: svg, viewBox: '0 0 ' + W + ' ' + H, width: '100%', role: 'img',
        'aria-label': '轨道图：' + nodes.map(function (n) { return n.name; }).join(', ') }, handlers),
        h('g', { transform: 'translate(' + view.x.toFixed(2) + ' ' + view.y.toFixed(2) + ') scale(' + k.toFixed(4) + ')', 'aria-hidden': true },
          h(LandDots, { left: left, top: topY, cell: cell, lon0: lon0 })),
        nodes.map(function (n, i) {
          var geo = n.lon != null ? [n.lon, n.lat] : geoOf(n.name);
          if (!geo && (n.x == null || n.y == null)) return null;
          var p = scr(geo ? at(geo[0], geo[1]) : [n.x * W, n.y * H]), nx = p[0], ny = p[1];
          // city-level stops, OpenTrace style: each located hop is its own marker, not one jump.
          // 12 px only merges the same city (or an unnamed hop); San Francisco stays beside Los Angeles.
          function sameCity(a, b) { return !a || !b || a === b; }
          var stops = [];
          (n.hops || (n.entry ? [{ at: n.entry }] : [])).forEach(function (hop) {
            if (!hop || !hop.at) return;
            var point = scr(at(hop.at[0], hop.at[1]));
            var place = hop.place || '';
            var last = stops[stops.length - 1];
            if (last && Math.hypot(point[0] - last.point[0], point[1] - last.point[1]) < 12 && sameCity(last.place, place)) return;
            if (Math.hypot(point[0] - nx, point[1] - ny) < 12 && sameCity(place, n.place || '')) return;
            if (o && Math.hypot(point[0] - o[0], point[1] - o[1]) < 12) return;
            stops.push({ point: point, place: place, country: hop.country });
          });
          var start = o || (stops[0] && stops[0].point), flip = nx > W - 140, f = flagOf(n.name);
          // A located address carries its measured country; the name's guessed flag then stays out,
          // so a node named 日本 that lands in the US does not show both.
          var placeFlag = n.place ? emojiOfCode(n.country) : '';
          function withFlag(place, country) { var e = emojiOfCode(country); return (e ? e + ' ' : '') + place; }
          function labelAt(x, y, align, lines) {
            return h('text', { x: x, y: y, textAnchor: align, className: 'lf-traj__label' },
              lines[0],
              lines.slice(1).map(function (line, index) {
                return line && h('tspan', { key: index, x: x, dy: 14, className: 'lf-traj__place' }, line);
              }));
          }
          var label = labelAt(flip ? nx - 8 : nx + 8, ny + 4, flip ? 'end' : 'start',
            [(f && !placeFlag ? f.emoji + ' ' : '') + n.name + (n.delay ? '  ' + n.delay + 'ms' : ''), n.place && withFlag(n.place, n.country)]);
          var node = h('circle', { cx: nx, cy: ny, r: n.active ? 4 : 3, className: 'lf-traj__node' });
          var markers = stops.map(function (stop, index) {
            var crowded = Math.hypot(stop.point[0] - nx, stop.point[1] - ny) < 28;
            var side = crowded ? !flip : stop.point[0] > W - 140;
            return h('g', { key: 'hop-' + index },
              h('circle', { cx: stop.point[0], cy: stop.point[1], r: 3.5, className: 'lf-traj__hop' },
                h('title', null, stop.place || (n.name + ' 第 ' + (index + 1) + ' 跳'))),
              stop.place && labelAt(side ? stop.point[0] - 8 : stop.point[0] + 8, stop.point[1] - 8, side ? 'end' : 'start', [withFlag(stop.place, stop.country)]));
          });
          if (!start) return h('g', { key: n.name, className: cx('lf-traj__route', n.active && 'is-active') }, node, label);
          function arc(a, b) {
            var dist = Math.hypot(b[0] - a[0], b[1] - a[1]);
            return ' Q' + ((a[0] + b[0]) / 2).toFixed(1) + ' ' + (Math.min(a[1], b[1]) - Math.max(18, dist * 0.32)).toFixed(1) + ' ' + b[0].toFixed(1) + ' ' + b[1].toFixed(1);
          }
          var points = (o ? [o] : []).concat(stops.map(function (stop) { return stop.point; }), [[nx, ny]]);
          var d = 'M' + points[0][0].toFixed(1) + ' ' + points[0][1].toFixed(1) + points.slice(1).map(function (point, index) { return arc(points[index], point); }).join('');
          return h('g', { key: n.name, className: cx('lf-traj__route', n.active && 'is-active') },
            // dotted like the map; busier routes flow faster instead of getting thicker
            h('path', { d: d, fill: 'none', className: 'lf-traj__path', style: n.active ? { animationDuration: (1.6 / Math.min(4, n.weight || 1)).toFixed(2) + 's' } : null }),
            // the craft's path changes while zooming; let it rejoin once the view settles
            n.active && !still && !drag.current && h('circle', { key: d, r: 2.5, className: 'lf-traj__craft' },
              h('animateMotion', { dur: (2.4 + i * 0.5) + 's', repeatCount: 'indefinite', path: d })),
            markers, node, label);
        }),
        o && h('g', { className: 'lf-traj__origin' },
          h('circle', { cx: o[0], cy: o[1], r: 9, fill: 'none' }),
          h('path', { d: 'M' + (o[0] - 14) + ' ' + o[1] + 'h28M' + o[0] + ' ' + (o[1] - 14) + 'v28' }),
          props.origin !== '' && h('text', { x: o[0] - 14, y: o[1] - 14, textAnchor: 'end', className: 'lf-traj__label' }, props.origin ? (emojiOfCode(props.originCountry) ? emojiOfCode(props.originCountry) + ' ' : '') + props.origin : '本机'))),
      zoomable && h('div', { className: 'lf-traj__zoom' },
        h(Button, { variant: 'ghost', size: 'sm', icon: 'plus', 'aria-label': '放大', disabled: k >= maxK - 0.01, onClick: function () { zoomAt(1.6, W / 2, H / 2, true); } }),
        h('span', { className: 'lf-traj__level', 'aria-live': 'polite' }, k.toFixed(1) + '×'),
        h(Button, { variant: 'ghost', size: 'sm', icon: 'minus', 'aria-label': '缩小', disabled: !zoomed, onClick: function () { zoomAt(1 / 1.6, W / 2, H / 2, true); } }),
        zoomed && h(Button, { variant: 'ghost', size: 'sm', icon: 'refresh', 'aria-label': '复位', title: '复位', onClick: function () { glideTo(HOME); } })),
      (props.caption || zoomable) && h('figcaption', { className: 'lf-traj__caption' },
        h('span', null, props.caption),
        zoomable && h('span', { className: 'lf-traj__hint' }, zoomed ? '拖动平移 · 双击放大 · ⇧双击缩小' : '双击或双指缩放 · ⌘ + 滚轮')),
      h(TrajectoryRoutes, { nodes: nodes }));
  }

  // One row per located node under the map: entry → exit, then the measured outbound line.
  // Measured automatically by the app; nothing here is a button.
  function TrajectoryRoutes(props) {
    var rows = props.nodes.filter(function (n) { return n.route || n.tracing || n.entryPlace; });
    if (!rows.length) return null;
    return h('div', { className: 'lf-traj__routes' },
      rows.map(function (n) {
        var r = n.route, segs = (r && r.segments) || [];
        var note = r && r.error ? r.error : n.tracing ? '正在测量去程…' : '';
        return h('section', { key: n.name, className: 'lf-traj__row' },
          h('div', { className: 'lf-traj__rowhead' },
            h('span', { className: 'lf-traj__rowname' }, h(Flag, { name: n.name }), n.name),
            n.relay && h('span', { className: 'lf-traj__tag' }, '中转'),
            h('span', { className: 'lf-traj__rowpath' }, [n.entryPlace && '入口 ' + n.entryPlace, n.place && '落地 ' + n.place].filter(Boolean).join(' → '))),
          segs.length > 0 && h(RouteStrip, { segments: segs }),
          !segs.length && n.tracing && h('div', { className: 'lf-traj__strip is-pending', 'aria-hidden': true }, [0, 1, 2, 3].map(function (i) {
            return h('span', { key: i, className: 'lf-traj__seg', style: { animationDelay: (i * 120) + 'ms' } }, h('span', { className: 'lf-traj__segbar' }));
          })),
          note && h('p', { className: 'lf-traj__note' }, note));
      }));
  }

  // The measured trace, replayed: one dot per real hop (a silent hop is ---), and a
  // packet that walks hop to hop at the pace the trace measured — each leg takes a
  // short fixed step plus time in proportion to that hop's latency gain, and the
  // whole run is longer for a slower route, so a sea crossing reads as a long haul.
  function hopTimeline(segs) {
    var all = [];
    segs.forEach(function (sg, si) {
      var n = sg.kind === 'unreached' ? sg.hops || 0 : Math.max(sg.hops || 1, 1), list = sg.hopList && sg.hopList.length === n ? sg.hopList : null;
      for (var j = 0; j < n; j++) {
        var hp = list ? list[j] : { ms: j === n - 1 && sg.ms ? sg.ms : -1 };
        all.push({ seg: si, idx: j, n: n, ip: hp.ip || '', place: hp.place || '', ms: hp.ms, silent: list ? hp.ms < 0 : false });
      }
    });
    // silent hops take the last answer before them, so time never runs backwards
    var last = 0;
    all.forEach(function (hp) { if (hp.ms > last) last = hp.ms; hp.at = last; });
    var step = 90, travel = Math.min(Math.max(last * 10, 500), 2600), clock = 0, prev = 0;
    all.forEach(function (hp) { clock += step + (last ? travel * (hp.at - prev) / last : 0); prev = hp.at; hp.time = clock; });
    all.forEach(function (hp, k) { hp.no = k + 1; hp.t = hp.time / clock; });
    return { hops: all, run: clock };
  }
  function hopTip(hp) {
    if (hp.silent) return '第 ' + hp.no + ' 跳 · --- 无应答';
    if (!hp.ip) return '第 ' + hp.no + ' 跳' + (hp.ms > 0 ? ' · ' + hp.ms + ' ms' : '');
    return ['第 ' + hp.no + ' 跳', hp.ip, hp.place, hp.ms >= 0 ? hp.ms + ' ms' : ''].filter(Boolean).join(' · ');
  }
  function RouteStrip(props) {
    var segs = props.segments, ref = useRef(null), wrap = useRef(null), tipState = useState(null), tip = tipState[0], setTip = tipState[1];
    var key = segs.map(function (sg) { return sg.label + ':' + sg.hops + ':' + sg.ms + ':' + (sg.hopList || []).map(function (hp) { return hp.ip + '/' + hp.ms; }).join(','); }).join('|');
    var line = useMemo(function () { return hopTimeline(segs); }, [key]);
    useEffect(function () {
      var root = ref.current;
      if (!root || reducedMotion() || !root.animate) return;
      var pause = 1200, cycle = line.run + pause, scale = line.run / cycle, anims = [];
      var bars = root.querySelectorAll('.lf-traj__segbar');
      segs.forEach(function (sg, si) {
        var hops = line.hops.filter(function (hp) { return hp.seg === si; });
        var before = line.hops[line.hops.indexOf(hops[0]) - 1];
        var from = (before ? before.t : 0) * scale, bar = bars[si], dot = bar && bar.querySelector('.lf-traj__packet');
        if (!dot || !hops.length) return;
        var at = function (hp) { return Math.max(hp.t * scale, from + 0.004); };
        var frames = [{ offset: 0, left: '0%', opacity: 0 }];
        if (from > 0) frames.push({ offset: from, left: '0%', opacity: 0 });
        frames.push({ offset: Math.min(from + 0.003, 1), left: '0%', opacity: 1 });
        hops.forEach(function (hp) { frames.push({ offset: at(hp), left: ((hp.idx + 0.5) / hp.n * 100) + '%', opacity: 1 }); });
        var end = at(hops[hops.length - 1]), stay = frames[frames.length - 1].left;
        frames.push({ offset: Math.min(end + 0.02, 1), left: stay, opacity: 0 });
        if (frames[frames.length - 1].offset < 1) frames.push({ offset: 1, left: stay, opacity: 0 });
        anims.push(dot.animate(frames, { duration: cycle, iterations: Infinity, easing: 'linear' }));
        var marks = bar.querySelectorAll('.lf-traj__hop');
        hops.forEach(function (hp, j) {
          var o = at(hp), mark = marks[j];
          if (!mark || hp.silent) return;
          var lit = [{ offset: 0, transform: 'scale(1)' }];
          if (o > 0.001) lit.push({ offset: o - 0.001, transform: 'scale(1)' });
          lit.push({ offset: o, transform: 'scale(1.8)' }, { offset: Math.min(o + 0.08, 1), transform: 'scale(1)' });
          if (lit[lit.length - 1].offset < 1) lit.push({ offset: 1, transform: 'scale(1)' });
          anims.push(mark.animate(lit, { duration: cycle, iterations: Infinity }));
        });
      });
      return function () { anims.forEach(function (a) { a.cancel(); }); };
    }, [line]);
    var show = function (hp) {
      return function (e) {
        var box = wrap.current && wrap.current.getBoundingClientRect(), r = e.currentTarget.getBoundingClientRect();
        if (!box) return;
        var x = r.left + r.width / 2 - box.left;
        if (box) setTip({ text: hopTip(hp), x: x, y: r.top - box.top, edge: x > box.width - 130 ? 'end' : x < 130 ? 'start' : '' });
      };
    };
    var hide = function () { setTip(null); };
    return h('div', { ref: wrap, className: 'lf-traj__stripwrap' },
      h('ol', { ref: ref, className: 'lf-traj__strip', 'aria-label': '去程线路' }, segs.map(function (sg, i) {
        var hops = line.hops.filter(function (hp) { return hp.seg === i; });
        return h('li', { key: i, className: cx('lf-traj__seg', 'is-' + (sg.kind || 'carrier')), style: sg.kind === 'entry' ? undefined : { flexGrow: sg.hops || 1, flexShrink: 0, flexBasis: 'auto' } },
          h('span', { className: 'lf-traj__seglabel' }, sg.label),
          h('span', { className: 'lf-traj__segbar' },
            hops.map(function (hp) {
              return h('i', { key: hp.idx, className: cx('lf-traj__hop', hp.silent && 'is-silent'), style: { left: ((hp.idx + 0.5) / hp.n * 100) + '%' },
                tabIndex: 0, 'aria-label': hopTip(hp), onMouseEnter: show(hp), onFocus: show(hp), onMouseLeave: hide, onBlur: hide });
            }),
            h('i', { className: 'lf-traj__packet', 'aria-hidden': true })),
          h('span', { className: 'lf-traj__segmeta' }, sg.kind === 'unreached' ? (sg.hops ? sg.hops + ' 跳沉默' : '') : sg.kind === 'entry' ? (sg.ms ? sg.ms + ' ms' : '') : (sg.hops || 1) + ' 跳' + (sg.ms ? ' · ' + sg.ms + ' ms' : '')));
      })),
      tip && h('div', { className: cx('lf-traj__hoptip', tip.edge && 'is-' + tip.edge), role: 'tooltip', style: { left: tip.x + 'px', top: tip.y + 'px' } }, tip.text));
  }

  /* ---------- ConnectionTable ---------- */

  // who owns the connection: the app's own icon when the backend sends one, else a neutral tile
  function connApp(c) {
    var name = c.app || c.process;
    if (c.icon) return h('img', { className: 'lf-conn__app', src: c.icon, alt: '' });
    // command-line tools (node, curl, apsd…) have no bundle icon; .app paths and capitalised names are apps
    var cli = name && (c.processPath ? !/\.app\//.test(c.processPath) : name.charAt(0) === name.charAt(0).toLowerCase() && !/[一-鿿]/.test(name.charAt(0)));
    return h('span', { className: 'lf-conn__app is-tile', 'aria-hidden': true },
      !name ? h(Icon, { name: 'globe', size: 12 }) : cli ? h(Icon, { name: 'terminal', size: 12 }) : name.charAt(0).toUpperCase());
  }

  // "DOMAIN-SUFFIX,github.com" → type and value, so a long value can shorten while the type stays readable.
  function connRule(rule) {
    var text = String(rule || ''), at = text.indexOf(',');
    return at < 0 ? { type: text, value: '' } : { type: text.slice(0, at).trim(), value: text.slice(at + 1).trim() };
  }
  // In the row: first group → +N → the node traffic leaves through. The whole chain is in the details.
  function connChain(chain) {
    var last = chain.length - 1, parts = chain.length > 3 ? [chain[0], null, chain[last]] : chain, hidden = chain.length - 2;
    return h('span', { className: 'lf-conn__chain', title: ['你'].concat(chain).join(' → ') }, parts.map(function (name, i) {
      var end = i === parts.length - 1;
      return h(React.Fragment, { key: i },
        i > 0 && h('span', { className: 'lf-conn__arrow', 'aria-hidden': true }, '→'),
        name == null ? h('span', { className: 'lf-conn__more' }, '+' + hidden)
          : h('span', { className: cx('lf-conn__hop', end && 'is-exit') }, end && h(Flag, { name: name }), name));
    }));
  }
  function connDetail(c, who) {
    function row(label, value) { return h(React.Fragment, { key: label }, h('dt', null, label), h('dd', null, value)); }
    return h('dl', { className: 'lf-conn__detail' },
      row('规则', c.rule || '—'),
      row('链路', h('ol', { className: 'lf-conn__path' }, ['你'].concat(c.chain || []).map(function (name, i, all) {
        return h('li', { key: i, className: cx(i === all.length - 1 && 'is-exit') }, name);
      }))),
      row('地址', c.host + (c.port ? ':' + c.port : '') + ' · ' + (c.network || 'TCP')),
      (who || c.processPath) && row('进程', c.processPath || who));
  }
  function ConnectionTable(props) {
    var rows = props.connections || [];
    var st = useState({}), open = st[0], setOpen = st[1];
    function toggle(id) { setOpen(function (cur) { var next = Object.assign({}, cur); if (next[id]) delete next[id]; else next[id] = true; return next; }); }
    return h('div', { className: cx('lf-table lf-conn', props.className), role: 'table', 'aria-label': '活动连接' },
      h('div', { className: 'lf-table__head', role: 'row' },
        ['连接', '流量', ''].map(function (c, i) { return h('span', { key: i, role: 'columnheader' }, c); })),
      rows.length === 0 && h('div', { className: 'lf-table__empty' }, props.empty || '暂无活动连接'),
      rows.map(function (c) {
        var who = c.app || c.process, rule = connRule(c.rule), expanded = !!open[c.id];
        function menu(e, el) {
          if (!props.onContextMenu) return;
          e.preventDefault();
          var r = (el || e.currentTarget).getBoundingClientRect();
          props.onContextMenu(c, e.clientX ? { x: e.clientX, y: e.clientY } : { x: r.left + 24, y: r.bottom });
        }
        return h('div', { key: c.id, className: cx('lf-table__row', expanded && 'is-open'), role: 'row', tabIndex: 0, 'aria-expanded': expanded,
          // details stay open for selecting text: only the row's own lines toggle
          onClick: function (e) {
            if (e.target.closest('button, .lf-conn__detail')) return;
            var picked = window.getSelection && window.getSelection();
            if (picked && !picked.isCollapsed) return;
            toggle(c.id);
          },
          onContextMenu: menu,
          onKeyDown: function (e) {
            if (e.key === 'ContextMenu' || (e.shiftKey && e.key === 'F10')) menu(e);
            else if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) { e.preventDefault(); toggle(c.id); }
          } },
          h('span', { role: 'cell', className: 'lf-conn__main' },
            h('span', { className: 'lf-conn__host', title: (c.processPath || who || '未知进程') + ' → ' + c.host + (c.port ? ':' + c.port : '') },
              connApp(c),
              who && h('span', { className: 'lf-conn__proc' }, who),
              who && h('span', { className: 'lf-conn__arrow', 'aria-hidden': true }, '→'),
              h('span', { className: 'lf-conn__name' }, c.host + (c.port ? ':' + c.port : '')),
              h('span', { className: 'lf-conn__net' }, c.network || 'TCP')),
            h('span', { className: 'lf-conn__route' },
              h('span', { className: 'lf-conn__rule', title: c.rule },
                h('span', { className: 'lf-conn__ruletype' }, rule.type),
                rule.value && h('span', { className: 'lf-conn__rulevalue' }, rule.value)),
              connChain(c.chain || []))),
          h('span', { role: 'cell', className: 'lf-conn__traffic' },
            h('span', { className: 'lf-num', 'aria-label': '下载 ' + fmt.bytes(c.down || 0) }, '↓ ' + fmt.bytes(c.down || 0)),
            h('span', { className: 'lf-num', 'aria-label': '上传 ' + fmt.bytes(c.up || 0) }, '↑ ' + fmt.bytes(c.up || 0))),
          h('span', { role: 'cell', className: 'lf-conn__acts' },
            h(Button, { variant: 'ghost', size: 'sm', icon: 'chevron', className: 'lf-conn__expand', 'aria-label': (expanded ? '收起详情：' : '展开详情：') + c.host, 'aria-expanded': expanded, onClick: function () { toggle(c.id); } }),
            props.onContextMenu && h(Button, { variant: 'ghost', size: 'sm', icon: 'more', 'aria-label': '更多操作：' + c.host, 'aria-haspopup': 'menu',
              onClick: function (e) { var r = e.currentTarget.getBoundingClientRect(); props.onContextMenu(c, { x: r.right - 200, y: r.bottom + 4 }); } }),
            props.onClose && h(Button, { variant: 'ghost', size: 'sm', icon: 'close', 'aria-label': '关闭连接：' + c.host, onClick: function () { props.onClose(c.id); } })),
          expanded && connDetail(c, who));
      }));
  }

  /* ---------- LogStream ---------- */

  var LEVELS = ['debug', 'info', 'warning', 'error'];
  var LEVEL_TAG = { debug: 'DBG', info: 'INF', warning: 'WRN', error: 'ERR' };

  function LogStream(props) {
    var lines = props.lines || [];
    var st = useState(props.level || 'info'), level = st[0], setLevel = st[1];
    var min = LEVELS.indexOf(level);
    var q = (props.query || '').toLowerCase();
    var shown = lines.filter(function (l) { return LEVELS.indexOf(l.level) >= min && (!q || l.message.toLowerCase().indexOf(q) >= 0); });
    return h('div', { className: cx('lf-logs', props.className) },
      props.showFilter !== false && h('div', { className: 'lf-logs__bar' },
        h(SegmentedControl, { size: 'sm', 'aria-label': '最低日志级别', value: level, onChange: setLevel, options: LEVELS.map(function (l) { return { value: l, label: LEVEL_TAG[l] }; }) }),
        h('span', { className: 'lf-logs__count' }, shown.length + ' / ' + lines.length)),
      h('ol', { className: 'lf-logs__list', 'aria-live': 'polite' },
        shown.map(function (l, i) {
          return h('li', { key: i, className: cx('lf-log', 'lf-log--' + l.level) },
            h('span', { className: 'lf-log__time' }, l.time),
            h('span', { className: 'lf-log__tag' }, LEVEL_TAG[l.level] || l.level),
            h('span', { className: 'lf-log__msg' }, l.message));
        })));
  }

  /* ---------- ProfileCard ---------- */

  function ProfileCard(props) {
    var u = props.usage, pct = u && u.total ? clamp(u.used / u.total, 0, 1) : null;
    return h('article', { className: cx('lf-profile', props.active && 'is-active', props.className) },
      h('div', { className: 'lf-profile__head' },
        h(Icon, { name: props.source === 'file' ? 'layers' : 'globe' }),
        h('div', { className: 'lf-profile__title' },
          h('div', { className: 'lf-profile__name' }, props.name),
          h('div', { className: 'lf-profile__src' }, props.source === 'file' ? '本地 YAML' : (props.url || '订阅'))),
        props.active ? h('span', { className: 'lf-tag' }, h(Icon, { name: 'check', size: 12 }), '使用中') : h(Button, { size: 'sm', onClick: props.onUse }, '使用')),
      pct != null && h('div', { className: 'lf-profile__usage' },
        h('div', { className: 'lf-meter', role: 'meter', 'aria-valuemin': 0, 'aria-valuemax': 100, 'aria-valuenow': Math.round(pct * 100), 'aria-label': '已用流量' },
          h('span', { style: { width: (pct * 100).toFixed(1) + '%' } })),
        h('div', { className: 'lf-profile__nums' }, h('span', null, fmt.bytes(u.used) + ' / ' + fmt.bytes(u.total)), props.expires && h('span', null, '到期 ' + props.expires))),
      h('div', { className: 'lf-profile__foot' },
        props.error ? h('span', { className: 'lf-profile__err', role: 'alert' }, h(Icon, { name: 'alert-circle', size: 12 }), props.error) : h('span', null, '更新于 ' + (props.updated || '—')),
        h('span', { className: 'lf-profile__actions' },
          props.onEdit && h(Button, { variant: 'ghost', size: 'sm', icon: 'edit', onClick: props.onEdit }, '编辑'),
          props.source !== 'file' && h(Button, { variant: 'ghost', size: 'sm', icon: 'refresh', className: props.updating ? 'is-spinning' : '', disabled: props.updating, onClick: props.onUpdate }, props.updating ? '更新中' : '更新订阅'))));
  }

  /* ---------- SearchField ---------- */

  function SearchField(props) {
    var rest = Object.assign({}, props); delete rest.className; delete rest.onValueChange;
    return h('label', { className: cx('lf-search', props.className) },
      h(Icon, { name: 'search' }),
      h('input', Object.assign({ type: 'search', placeholder: '筛选' }, rest, {
        onChange: function (e) { if (props.onChange) props.onChange(e); if (props.onValueChange) props.onValueChange(e.target.value); }
      })),
      props.shortcut && h('kbd', null, props.shortcut));
  }

  /* ---------- Sidebar ---------- */

  function Sidebar(props) {
    var items = props.items || [];
    var ind = useRef(null), refs = useRef({}), first = useRef(true);
    var b = useBody(0, springs.glide, function (y) { if (ind.current) ind.current.style.transform = 'translateY(' + y + 'px)'; }, 0.05);
    useLayoutEffect(function () {
      var el = refs.current[props.active]; if (!el) return;
      if (first.current) { b.set(el.offsetTop); first.current = false; } else b.to(el.offsetTop);
      if (ind.current) ind.current.style.height = el.offsetHeight + 'px';
    }, [props.active]);
    return h('nav', { className: cx('lf-sidebar', props.className), 'aria-label': '主导航' },
      h('div', { className: 'lf-sidebar__chrome', 'aria-hidden': true }),
      h('div', { className: 'lf-sidebar__brand' }, h(Icon, { name: 'rocket' }), h('span', null, props.title || 'Rocket Science')),
      h('div', { className: 'lf-sidebar__list' },
        h('span', { className: 'lf-sidebar__ind', ref: ind, 'aria-hidden': true }),
        items.map(function (it) {
          var act = it.id === props.active;
          return h('button', {
            key: it.id, type: 'button', 'data-nav': it.id, className: cx('lf-sidebar__item', act && 'is-active'), 'aria-current': act ? 'page' : undefined,
            ref: function (el) { refs.current[it.id] = el; },
            onClick: function () { if (props.onSelect) props.onSelect(it.id); }
          }, h(Icon, { name: it.icon }), h('span', null, it.label), it.badge != null && h('span', { className: 'lf-sidebar__badge' }, it.badge));
        }).reduce(function (out, btn, k) {
          // the active item's topics open right below it; only rendered while active so later items never sit on a collapsing list
          var it = items[k], kids = it.children;
          out.push(btn);
          if (kids && kids.length && it.id === props.active) out.push(h('div', { key: it.id + ':sub', className: 'lf-sidebar__sub', role: 'group', 'aria-label': it.label },
            kids.map(function (c, j) {
              // a change of group is marked by a hairline gap, not a heading
              var split = j > 0 && c.group && kids[j - 1].group !== c.group;
              var on = c.id === props.activeChild;
              return [split && h('div', { key: 'g' + j, className: 'lf-sidebar__split', role: 'separator' }),
                h('button', { key: c.id, type: 'button', 'data-nav': it.id + '/' + c.id, className: cx('lf-sidebar__subitem', on && 'is-active'), 'aria-current': on ? 'page' : undefined,
                  onClick: function () { if (props.onSelectChild) props.onSelectChild(it.id, c.id); } }, c.label)];
            })));
          return out;
        }, [])),
      props.footer && h('div', { className: 'lf-sidebar__foot' }, props.footer));
  }

  /* ---------- TrayIndicator + MenuBarPopover: the menu-bar extra ---------- */

  // the unit is picked after rounding, so 999.6K reads 1.0M, never the five-character 1000K
  function shortNumber(bps, i) {
    var fixed = bps.toFixed(1);
    return i > 0 && fixed.length <= 3 ? fixed : String(Math.round(bps));
  }

  function shortSpeed(bps) {
    if (bps == null || isNaN(bps)) return '—';
    var u = ['B', 'K', 'M', 'G'], i = 0, n = shortNumber(bps, 0);
    while (n.length > 3 && i < u.length - 1) { bps /= 1024; i++; n = shortNumber(bps, i); }
    return n + u[i];
  }

  function useClock(active) {
    var st = useState(Date.now());
    useEffect(function () {
      if (!active) return;
      var t = setInterval(function () { st[1](Date.now()); }, 1000);
      return function () { clearInterval(t); };
    }, [active]);
    return st[0];
  }

  var TRAY_LAP = 1600;

  function TrayIndicator(props) {
    var display = props.display || 'both', state = props.state || 'idle';
    var live = state === 'orbit';
    // reaching orbit: the satellite circles three times, then rests as a still dot; hovering replays it
    var lap = useState(false);
    function replay() { if (live && !reducedMotion()) lap[1](Date.now()); }
    useEffect(function () { if (live) replay(); else lap[1](false); }, [live]);
    useEffect(function () { if (!lap[0]) return; var t = setTimeout(function () { lap[1](false); }, TRAY_LAP * 3); return function () { clearTimeout(t); }; }, [lap[0]]);
    return h('button', {
      type: 'button', className: cx('lf-tray', 'is-' + state, props.active && 'is-active', !live && 'is-idle', props.className),
      onClick: props.onClick, onPointerEnter: replay, 'aria-label': 'Rocket Science 菜单', 'aria-expanded': !!props.active
    },
      display !== 'speed' && h('span', { className: 'lf-tray__mark', 'aria-hidden': true },
        h(Icon, { name: 'rocket', size: 14 }),
        state === 'igniting' && h('span', { className: 'lf-tray__flame' }),
        live && h('span', { key: lap[0] || 'rest', className: cx('lf-tray__orbit', lap[0] && 'is-playing') }, h('span', { className: 'lf-tray__sat' }))),
      // speeds only while in orbit: idle 'both' collapses to the bare mark instead of showing dashes
      display !== 'icon' && (live || display === 'speed') && h('span', { className: 'lf-tray__speed' },
        h('span', null, h('i', null, '↑'), shortSpeed(live ? props.up : null)),
        h('span', null, h('i', null, '↓'), shortSpeed(live ? props.down : null))));
  }

  var POP_SHORTCUTS = { s: 'systemProxy', e: 'tun', c: 'copy', d: 'dashboard' };

  function MenuBarPopover(props) {
    var groups = props.groups || [];
    var gs = useState(props.defaultGroup || (groups[0] && groups[0].name));
    var group = groups.find(function (g) { return g.name === gs[0]; }) || groups[0];
    var ns = useNameSort(props);
    var state = props.state || 'idle', live = state === 'orbit', igniting = state === 'igniting';
    var copy = Object.assign({}, IGNITION_COPY[state], (props.copy || {})[state]);
    var now = useClock(live && !!props.since);
    var cp = useState(false), copied = cp[0];
    var root = useRef(null), open = props.open !== false, copying = useRef(false), copyTimer = useRef(null), copyMounted = useRef(true);
    useEffect(function () { copyMounted.current = true; return function () { copyMounted.current = false; if (copyTimer.current) clearTimeout(copyTimer.current); }; }, []);

    var body = useBody(open ? 1 : 0, springs.glide, function (p) {
      var el = root.current; if (!el) return;
      el.style.opacity = clamp(p * 1.5, 0, 1).toFixed(3);
      el.style.transform = p >= 1 ? '' : 'translateY(' + ((1 - p) * -6).toFixed(2) + 'px) scale(' + (0.97 + 0.03 * p).toFixed(4) + ')';
      el.style.visibility = p < 0.02 && body.target === 0 ? 'hidden' : 'visible';
    }, 0.001);
    useLayoutEffect(function () { body.emit(); }, []);
    useEffect(function () { body.to(open ? 1 : 0); }, [open]);

    function doCopy() {
      if (!props.onCopyCommand || copying.current) return;
      copying.current = true;
      if (copyTimer.current) clearTimeout(copyTimer.current);
      cp[1](false);
      Promise.resolve().then(function () { return props.onCopyCommand(); }).then(function () {
        if (!copyMounted.current) return;
        cp[1](true); copyTimer.current = setTimeout(function () { cp[1](false); }, 1500);
      }).catch(function () { if (copyMounted.current) cp[1](false); }).finally(function () { copying.current = false; });
    }
    useEffect(function () {
      if (!open) return;
      function onKey(e) {
        if (!e.metaKey) return;
        var k = POP_SHORTCUTS[e.key.toLowerCase()]; if (!k) return;
        e.preventDefault();
        if (k === 'systemProxy' && props.onSystemProxyChange) props.onSystemProxyChange(!props.systemProxy);
        if (k === 'tun' && props.onTunChange) props.onTunChange(!props.tun);
        if (k === 'copy') doCopy();
        if (k === 'dashboard' && props.onOpenDashboard) props.onOpenDashboard();
      }
      window.addEventListener('keydown', onKey);
      return function () { window.removeEventListener('keydown', onKey); };
    }, [open, props.systemProxy, props.tun, props.onSystemProxyChange, props.onTunChange, props.onCopyCommand, props.onOpenDashboard]);

    var sp = props.speed || {};
    function speedCell(dir) {
      var hist = (sp.history || {})[dir] || [], v = fmt.speed(live ? sp[dir] : null);
      var W = 100, H = 20, max = Math.max.apply(null, hist.concat([1]));
      var pts = hist.map(function (x, i) { return (i / Math.max(1, hist.length - 1) * W).toFixed(1) + ',' + (H - 1 - (x / max) * (H - 2)).toFixed(1); }).join(' ');
      return h('div', { className: 'lf-pop__cell' },
        h('div', { className: 'lf-pop__cap' }, h(Icon, { name: dir === 'up' ? 'arrow-up' : 'arrow-down', size: 12 }), dir === 'up' ? '上行' : '下行'),
        h('div', { className: 'lf-pop__num' }, v.value, h('small', null, v.unit)),
        hist.length > 1 && h('svg', { className: 'lf-pop__wave', viewBox: '0 0 ' + W + ' ' + H, preserveAspectRatio: 'none', 'aria-hidden': true },
          h('polyline', { points: pts, fill: 'none', stroke: 'currentColor', strokeWidth: 1.25, vectorEffect: 'non-scaling-stroke', strokeLinejoin: 'round' })));
    }
    function toggleRow(label, kbd, checked, onChange, desc) {
      return h('div', { className: 'lf-pop__toggle' },
        h('span', { className: 'lf-pop__tlabel' }, label, desc && h('small', null, desc)),
        kbd && h('kbd', null, kbd),
        h(Switch, { checked: !!checked, onChange: onChange, disabled: !onChange, 'aria-label': label }));
    }
    function menuItem(icon, label, kbd, onClick) {
      return h('button', { type: 'button', className: 'lf-pop__item', onClick: onClick },
        h(Icon, { name: icon }), h('span', null, label), kbd && h('kbd', null, kbd));
    }

    var testing = group && props.testing === group.name;
    var readOnly = group && (group.readOnly || (group.type && group.type !== 'Selector'));
    return h('div', { ref: root, className: cx('lf-pop', props.className), role: 'dialog', 'aria-label': 'Rocket Science', 'aria-hidden': !open || undefined },
      h('div', { className: 'lf-pop__head' },
        h('span', { className: cx('lf-pop__beacon', live && 'is-live', igniting && 'is-igniting'), 'aria-hidden': true }),
        h('div', { className: 'lf-pop__status' },
          h('div', { className: cx('lf-pop__title', live && 'is-live') }, copy.title),
          h('div', { className: 'lf-pop__sub' }, live && props.since ? fmt.clock((now - props.since) / 1000) + (props.subtitle ? ' · ' + props.subtitle : '') : (props.subtitle || ''))),
        h(Button, {
          size: 'sm', variant: live ? 'secondary' : 'primary', icon: live ? 'power' : 'rocket', disabled: igniting || (!live && !props.systemProxy && !props.tun),
          title: !live && !props.systemProxy && !props.tun ? '至少选择一种接管方式' : undefined,
          onClick: function () { if (props.onToggle) props.onToggle(live ? 'land' : 'launch'); }
        }, copy.action)),
      h('div', { className: 'lf-pop__speed' }, speedCell('down'), speedCell('up')),
      h('div', { className: 'lf-pop__section' },
        h(SegmentedControl, { className: 'lf-pop__mode', 'aria-label': '出站模式', value: props.mode, onChange: props.onModeChange, disabled: !live, options: [
          { value: 'rule', label: '规则' }, { value: 'global', label: '全局' }, { value: 'direct', label: '直连' }] })),
      group && h('div', { className: 'lf-pop__section lf-pop__proxies' },
        h('div', { className: 'lf-pop__groups' },
          // the chips scroll; sort and 测速 stay pinned on the right
          h('div', { className: 'lf-pop__chips', role: 'tablist', 'aria-label': '策略组' },
            groups.map(function (g) {
              var sel = g === group;
              return h('button', { key: g.name, type: 'button', role: 'tab', 'aria-selected': sel, className: cx('lf-chip', sel && 'is-selected'), onClick: function () { gs[1](g.name); } }, g.name);
            })),
          group.nodes.length > 1 && h(SortMenu, { value: ns[0], onChange: ns[1], className: 'lf-pop__sort' }),
          h(Button, { variant: 'ghost', size: 'sm', icon: 'refresh', className: cx('lf-pop__test', testing && 'is-spinning'), disabled: igniting || !!props.testing || !props.onTest, 'aria-label': '测速：' + group.name, onClick: function () { if (props.onTest) props.onTest(group.name); } })),
        h('div', { className: 'lf-pop__nodes', role: 'radiogroup', 'aria-label': group.name },
          sortNodes(group.nodes, ns[0]).map(function (n) {
            var sel = n.name === group.now;
            return h('button', {
              key: n.name, type: 'button', role: 'radio', 'aria-checked': sel, 'aria-disabled': readOnly || igniting || undefined, disabled: igniting,
              className: cx('lf-pop__node', sel && 'is-selected', readOnly && 'is-readonly'),
              onClick: function () { if (!readOnly && props.onSelect) props.onSelect(group.name, n.name); }
            }, h('span', { className: 'lf-node__dot' }), h('span', { className: 'lf-pop__nodename' }, h(Flag, { name: n.name }), n.name), h('span', { className: 'lf-pop__nodetype' }, n.type), h(LatencyBadge, { ms: n.delay, testing: testing }));
          }))),
      h('div', { className: 'lf-pop__section' },
        h('div', { className: 'lf-pop__cap lf-pop__caphead' }, '接管方式', !live && (props.systemProxy || props.tun ? h('span', null, '启动后生效') : h('span', { className: 'is-warn' }, '至少选择一种接管方式'))),
        toggleRow('系统代理', '⌘S', props.systemProxy, props.onSystemProxyChange, takeoverNote(state, props.systemProxy)),
        toggleRow('TUN 增强模式', '⌘E', props.tun, props.onTunChange, props.onTunChange ? takeoverNote(state, props.tun) : '尚未接入'),
        props.onSsidBypassChange && toggleRow('可信 Wi-Fi 直连', null, props.ssidBypass, props.onSsidBypassChange, props.ssid ? '当前网络：' + props.ssid : null)),
      h('div', { className: 'lf-pop__section lf-pop__menu' },
        menuItem(copied ? 'check' : 'copy', copied ? '已复制到剪贴板' : '复制终端代理命令', '⌘C', doCopy),
        menuItem('gauge', '打开控制台', '⌘D', props.onOpenDashboard),
        props.onQuit && menuItem('power', '退出 Rocket Science', '⌘Q', props.onQuit)));
  }

  /* ---------- Takeover: 启动 runs the core, the switches choose how traffic reaches it ---------- */

  // the switches are preferences: while stopped they only record the choice; 启动 applies them, 停止 undoes them
  function takeoverNote(state, on, detail) {
    if (!on) return '';
    if (state === 'orbit') return '已接管' + (detail ? ' · ' + detail : '');
    if (state === 'igniting') return '正在接管…';
    return '';
  }
  function takeoverSummary(sys, tun) {
    var ways = [sys && '系统代理', tun && 'TUN'].filter(Boolean);
    return ways.length ? ways.join(' + ') : '未接管';
  }

  /* ---------- Tour: a spotlight walk-through for first-time users ---------- */

  function Tour(props) {
    var steps = props.steps || [];
    var st = useState(0), i = props.step != null ? props.step : st[0];
    var step = steps[i] || {}, last = i === steps.length - 1, solo = steps.length === 1;
    var open = props.open !== false && steps.length > 0;
    var hole = useRef(null), card = useRef(null), blockers = useRef([]);
    var rect = useRef({ x: 0, y: 0, w: 0, h: 0 }), first = useRef(true);
    function paint() {
      var r = rect.current, el = hole.current, c = card.current; if (!el || !c) return;
      el.style.transform = 'translate(' + r.x.toFixed(1) + 'px,' + r.y.toFixed(1) + 'px)';
      el.style.width = Math.max(0, r.w) + 'px'; el.style.height = Math.max(0, r.h) + 'px';
      var vw = window.innerWidth, vh = window.innerHeight;
      // Four blockers around the hole: the page stays inert except the spotlit element.
      var b = blockers.current;
      if (b[0]) { b[0].style.cssText = 'left:0;top:0;width:' + vw + 'px;height:' + Math.max(0, r.y) + 'px'; }
      if (b[1]) { b[1].style.cssText = 'left:0;top:' + (r.y + r.h) + 'px;width:' + vw + 'px;height:' + Math.max(0, vh - r.y - r.h) + 'px'; }
      if (b[2]) { b[2].style.cssText = 'left:0;top:' + r.y + 'px;width:' + Math.max(0, r.x) + 'px;height:' + Math.max(0, r.h) + 'px'; }
      if (b[3]) { b[3].style.cssText = 'left:' + (r.x + r.w) + 'px;top:' + r.y + 'px;width:' + Math.max(0, vw - r.x - r.w) + 'px;height:' + Math.max(0, r.h) + 'px'; }
      var cw = c.offsetWidth, ch = c.offsetHeight, gap = 16, x, y;
      if (!step.target) { x = (vw - cw) / 2; y = (vh - ch) / 2; }
      else {
        var pl = step.placement;
        if (!pl) pl = r.x + r.w + gap + cw < vw - 16 ? 'right' : r.y + r.h + gap + ch < vh - 16 ? 'bottom' : r.x - gap - cw > 16 ? 'left' : 'top';
        if (pl === 'right') { x = r.x + r.w + gap; y = r.y; }
        else if (pl === 'left') { x = r.x - cw - gap; y = r.y; }
        else if (pl === 'bottom') { x = r.x; y = r.y + r.h + gap; }
        else { x = r.x; y = r.y - ch - gap; }
      }
      c.style.transform = 'translate(' + clamp(x, 16, vw - cw - 16).toFixed(1) + 'px,' + clamp(y, 16, vh - ch - 16).toFixed(1) + 'px)';
    }
    var bx = useBody(0, springs.glide, function (v) { rect.current.x = v; paint(); }, 0.05);
    var by = useBody(0, springs.glide, function (v) { rect.current.y = v; paint(); }, 0.05);
    var bw = useBody(0, springs.glide, function (v) { rect.current.w = v; paint(); }, 0.05);
    var bh = useBody(0, springs.glide, function (v) { rect.current.h = v; paint(); }, 0.05);

    function measure(jump, scrollToTarget) {
      var pad = step.padding != null ? step.padding : 6, t = step.target && document.querySelector(step.target), r;
      if (t) {
        if (scrollToTarget !== false) t.scrollIntoView({ block: 'nearest', inline: 'nearest' });
        var b = t.getBoundingClientRect();
        r = { x: b.left - pad, y: b.top - pad, w: b.width + pad * 2, h: b.height + pad * 2 };
      } else r = { x: window.innerWidth / 2, y: window.innerHeight / 2, w: 0, h: 0 };
      [[bx, r.x], [by, r.y], [bw, r.w], [bh, r.h]].forEach(function (p) { if (jump) p[0].set(p[1]); else p[0].to(p[1]); });
    }
    useLayoutEffect(function () {
      if (!open) { first.current = true; return; }
      // Two frames: the host may switch views in onStepChange before the target exists.
      var a = requestAnimationFrame(function () {
        a = requestAnimationFrame(function () { measure(first.current); first.current = false; var btn = card.current && card.current.querySelector('.lf-tour__nav .lf-btn:last-child'); if (btn) btn.focus({ preventScroll: true }); });
      });
      return function () { cancelAnimationFrame(a); };
    }, [i, open]);
    useEffect(function () {
      if (!open) return;
      function onResize() { measure(true, false); }
      function target() { return step.target && document.querySelector(step.target); }
      function focusAreas() {
        var areas = [target(), card.current].filter(Boolean);
        areas.slice().forEach(function (area) {
          Array.prototype.forEach.call(area.querySelectorAll('[aria-controls]'), function (trigger) {
            (trigger.getAttribute('aria-controls') || '').split(/\s+/).forEach(function (id) {
              var popup = document.getElementById(id); if (popup && areas.indexOf(popup) < 0) areas.push(popup);
            });
          });
        });
        return areas;
      }
      function focusables() {
        var selector = 'button:not([disabled]),a[href],input:not([disabled]),textarea:not([disabled]),select:not([disabled]),[tabindex]:not([tabindex="-1"])';
        var result = [];
        focusAreas().forEach(function (area) {
          if (!area) return;
          var elements = Array.prototype.slice.call(area.querySelectorAll(selector));
          if (area.matches && area.matches(selector)) elements.unshift(area);
          elements.forEach(function (el) {
            if (el.tabIndex >= 0 && el.getClientRects().length && !el.closest('[hidden],[inert],[aria-hidden="true"]') && result.indexOf(el) < 0) result.push(el);
          });
        });
        return result;
      }
      function onFocus(e) {
        if (card.current && !focusAreas().some(function (area) { return area.contains(e.target); })) {
          var first = focusables()[0]; if (first) first.focus({ preventScroll: true });
        }
      }
      function onKey(e) {
        if (e.defaultPrevented) return;
        if (e.key === 'Escape') { e.preventDefault(); close(false); return; }
        if (e.key === 'Tab') {
          var list = focusables(), at = list.indexOf(document.activeElement);
          if (list.length) { e.preventDefault(); list[(at < 0 ? (e.shiftKey ? list.length - 1 : 0) : (at + (e.shiftKey ? -1 : 1) + list.length) % list.length)].focus({ preventScroll: true }); }
          return;
        }
        // Arrow keys on real controls belong to those controls, not the tour.
        if (e.altKey || e.ctrlKey || e.metaKey || (card.current && !card.current.contains(e.target))) return;
        if (e.key === 'ArrowRight') { e.preventDefault(); last ? close(true) : go(i + 1); }
        else if (e.key === 'ArrowLeft' && i > 0) { e.preventDefault(); go(i - 1); }
      }
      window.addEventListener('resize', onResize); window.addEventListener('scroll', onResize, true); window.addEventListener('keydown', onKey); document.addEventListener('focusin', onFocus);
      var observer = window.ResizeObserver ? new window.ResizeObserver(onResize) : null;
      if (observer) { if (target()) observer.observe(target()); if (card.current) observer.observe(card.current); }
      // A step may ask the person to try the real control: clicking it moves on.
      var t = step.advanceOnClick && step.target && document.querySelector(step.target), timer;
      function onTry() { timer = setTimeout(function () { last ? close(true) : go(i + 1); }, step.advanceDelay || 900); }
      if (t) t.addEventListener('click', onTry);
      return function () {
        window.removeEventListener('resize', onResize); window.removeEventListener('scroll', onResize, true); window.removeEventListener('keydown', onKey); document.removeEventListener('focusin', onFocus);
        if (observer) observer.disconnect();
        if (t) t.removeEventListener('click', onTry); clearTimeout(timer);
      };
    }, [open, i, step, props.onStepChange, props.onClose]);

    function go(n) {
      n = clamp(n, 0, steps.length - 1);
      if (props.step == null) st[1](n);
      if (props.onStepChange) props.onStepChange(n, steps[n]);
    }
    function close(done) {
      if (props.step == null) st[1](0);
      if (props.onClose) props.onClose(done);
    }

    if (!open) return null;
    return h('div', { className: 'lf-tour' },
      [0, 1, 2, 3].map(function (k) { return h('div', { key: k, className: 'lf-tour__block', ref: function (el) { blockers.current[k] = el; } }); }),
      h('div', { className: cx('lf-tour__hole', !step.target && 'is-empty'), ref: hole, 'aria-hidden': true }),
      h('div', { className: 'lf-tour__card', ref: card, role: 'dialog', 'aria-labelledby': 'lf-tour-title' },
        !solo && h('div', { className: 'lf-tour__progress', 'aria-hidden': true }, steps.map(function (_, k) { return h('span', { key: k, className: k <= i ? 'is-on' : '' }); })),
        h('div', { className: 'lf-tour__content', key: i },
          !solo && h('div', { className: 'lf-tour__count' }, (i + 1) + ' / ' + steps.length + (step.kicker ? ' · ' + step.kicker : '')),
          h('h2', { className: 'lf-tour__title', id: 'lf-tour-title' }, step.title),
          step.media && h('div', { className: 'lf-tour__media' }, step.media),
          h('div', { className: 'lf-tour__body' }, step.body),
          step.hint && h('div', { className: 'lf-tour__hint' }, h(Icon, { name: 'arrow-right', size: 12 }), step.hint)),
        h('div', { className: 'lf-tour__foot' },
          !last ? h(Button, { variant: 'ghost', size: 'sm', onClick: function () { close(false); } }, props.skipLabel || '跳过引导') : h('span'),
          h('div', { className: 'lf-tour__nav' },
            i > 0 && h(Button, { size: 'sm', onClick: function () { go(i - 1); } }, '上一步'),
            h(Button, { variant: solo ? 'ghost' : 'primary', size: 'sm', iconAfter: solo ? undefined : last ? 'check' : 'arrow-right', onClick: function () { last ? close(true) : go(i + 1); } },
              last ? (props.doneLabel || '开始使用') : '下一步')))));
  }

  function FirstRunCard(props) {
    var topics = props.topics || ['导入配置', '选择接管并启动', '测速选节点', '检查网络与模式'];
    return h('section', { className: cx('lf-firstrun', props.className), 'aria-label': '新手引导' },
      h('div', { className: 'lf-firstrun__icon' }, h(Icon, { name: 'rocket', size: 20 })),
      h('div', { className: 'lf-firstrun__text' },
        h('div', { className: 'lf-firstrun__title' }, props.title || '第一次使用？'),
        h('div', { className: 'lf-firstrun__desc' }, props.description || '约 2 分钟学会基本操作。开始前会先关闭代理，引导过程中不会断网。'),
        h('ol', { className: 'lf-firstrun__topics' }, topics.map(function (t, k) { return h('li', { key: k }, h('span', null, k + 1), t); }))),
      h('div', { className: 'lf-firstrun__actions' },
        h(Button, { variant: 'primary', icon: 'help', onClick: props.onStart }, props.startLabel || '开始引导'),
        props.onDismiss && h(Button, { variant: 'ghost', size: 'sm', onClick: props.onDismiss }, '以后再说')));
  }

  /* ---------- ConnectivityCheck: is the network actually working? ---------- */

  var CHECK_ICON = { ok: 'check', fail: 'close', running: 'refresh' };

  function ConnectivityCheck(props) {
    var checks = props.checks || [];
    var running = checks.some(function (c) { return c.status === 'running'; });
    var failed = checks.some(function (c) { return c.status === 'fail'; });
    var allOk = checks.some(function (c) { return c.status === 'ok'; }) && checks.every(function (c) { return c.status === 'ok' || c.status === 'skip'; });
    var tone = props.recovered ? 'idle' : failed ? 'fail' : allOk ? 'ok' : 'idle', compact = !!props.compact;
    var summary = props.recovered ? '已停止代理，已恢复原有网络设置' : running ? '正在检查…' : failed ? '发现问题：按提示处理，或先停止代理恢复网络' : allOk ? '一切正常，可以开始上网' : (props.idleText || '启动后检查一下网络是否真的连通');
    var runLabel = failed || allOk ? '重新检查' : '开始检查';
    if (compact) summary = props.recovered ? '已恢复原有设置' : running ? '检查中…' : failed ? '发现问题，按提示处理' : allOk ? '一切正常' : (props.idleText || '确认网络是否连通');
    var recoverBtn = (failed || running) && props.onRecover && h(Button, { size: 'sm', icon: 'power', className: compact ? 'lf-check__recover' : null, onClick: props.onRecover }, '停止代理，恢复网络');
    return h('section', { className: cx('lf-check', compact && 'lf-check--compact', 'is-' + tone, props.className), 'aria-label': props.title || '网络检查' },
      h('div', { className: 'lf-check__head' },
        h('div', { className: 'lf-check__text' },
          h('div', { className: 'lf-check__title' }, props.title || '网络检查'),
          h('div', { className: 'lf-check__summary', role: 'status' }, summary)),
        !compact && recoverBtn,
        compact
          ? h(Button, { size: 'sm', variant: 'ghost', icon: 'pulse', className: running ? 'is-spinning' : null, disabled: running, 'aria-label': runLabel, title: runLabel, onClick: props.onRun })
          : h(Button, { size: 'sm', variant: failed || allOk ? 'secondary' : 'primary', icon: 'pulse', disabled: running, onClick: props.onRun }, runLabel)),
      h('ol', { className: 'lf-check__list' }, checks.map(function (c) {
        var st = c.status || 'idle';
        return h('li', { key: c.id, className: cx('lf-check__item', 'is-' + st) },
          h('span', { className: cx('lf-check__icon', st === 'running' && 'is-spinning'), 'aria-hidden': true }, CHECK_ICON[st] && h(Icon, { name: CHECK_ICON[st], size: 12 })),
          h('span', { className: 'lf-check__label' }, c.label),
          h('span', { className: 'lf-check__detail', title: c.detail }, c.detail || (st === 'skip' ? '已跳过' : st === 'running' ? '检查中' : '—')),
          st === 'fail' && c.action && h(Button, { size: 'sm', variant: 'primary', className: 'lf-check__fix', onClick: c.action.onClick }, c.action.label));
      })),
      compact && recoverBtn);
  }

  /* ---------- CheckPopover: one quiet row; the full check opens in a small window ---------- */

  function CheckPopover(props) {
    var own = useState(false), open = props.open != null ? props.open : own[0];
    var trig = useRef(null), pop = useRef(null), at = useState(null);
    var checks = props.checks || [];
    var running = checks.some(function (c) { return c.status === 'running'; });
    var failed = !props.recovered && checks.some(function (c) { return c.status === 'fail'; });
    var allOk = checks.some(function (c) { return c.status === 'ok'; }) && checks.every(function (c) { return c.status === 'ok' || c.status === 'skip'; });
    var tone = failed ? 'fail' : running ? 'running' : allOk && !props.recovered ? 'ok' : 'idle';
    var state = props.recovered ? '已恢复' : { fail: '有问题', running: '检查中', ok: '正常', idle: '未检查' }[tone];
    function set(v) {
      if (props.open == null) own[1](v);
      if (props.onOpenChange) props.onOpenChange(v);
      if (!v && trig.current) trig.current.focus();
    }
    useLayoutEffect(function () {
      if (!open || !trig.current) return;
      var r = trig.current.getBoundingClientRect();
      // beside the trigger, bottom-aligned (it lives at the foot of the sidebar)
      at[1]({ left: r.right + 12, bottom: Math.max(8, window.innerHeight - r.bottom - 4) });
    }, [open]);
    useEffect(function () {
      if (!open) return;
      function away(e) { if (e.target && e.target.closest && e.target.closest('.lf-tour__card')) return; if (pop.current && !pop.current.contains(e.target) && !trig.current.contains(e.target)) set(false); }
      function esc(e) { if (e.key === 'Escape') set(false); }
      document.addEventListener('pointerdown', away, true); document.addEventListener('keydown', esc);
      var first = pop.current && pop.current.querySelector('button'); if (first) first.focus();
      return function () { document.removeEventListener('pointerdown', away, true); document.removeEventListener('keydown', esc); };
    }, [open]);
    var rest = Object.assign({}, props, { compact: true, className: 'lf-chkpop__check' });
    return h('div', { className: cx('lf-chkpop', props.className) },
      h('button', { type: 'button', ref: trig, className: cx('lf-chkpop__trigger', 'is-' + tone, open && 'is-open'), 'aria-haspopup': 'dialog', 'aria-expanded': open,
        onClick: function () { set(!open); } },
        h(Icon, { name: 'pulse', size: 14, className: running ? 'is-spinning' : null }),
        h('span', { className: 'lf-chkpop__label' }, props.title || '网络检查'),
        h('span', { className: 'lf-chkpop__state' }, h('i', { 'aria-hidden': true }), state)),
      open && h('div', { ref: pop, className: 'lf-chkpop__panel', role: 'dialog', 'aria-label': props.title || '网络检查',
        style: at[0] ? { left: at[0].left + 'px', bottom: at[0].bottom + 'px' } : { visibility: 'hidden' } },
        h(ConnectivityCheck, rest)));
  }

  /* ---------- Settings: sections of labelled rows, fields that commit on blur ---------- */

  function SettingsSection(props) {
    return h('section', { className: cx('lf-set', props.className), 'aria-label': props.title, id: props.id },
      h('header', { className: 'lf-set__head' },
        h('div', { className: 'lf-set__titles' },
          h('h2', { className: 'lf-set__title' }, props.title),
          props.description && h('p', { className: 'lf-set__desc' }, props.description)),
        props.status && h('span', { className: 'lf-set__status', role: 'status' }, h(Icon, { name: 'check', size: 12 }), props.status),
        props.action),
      h('div', { className: 'lf-set__body' }, props.children));
  }

  function SettingRow(props) {
    return h('div', { className: cx('lf-setrow', props.stack && 'is-stack', props.disabled && 'is-disabled', props.className) },
      h('div', { className: 'lf-setrow__text' },
        h('div', { className: 'lf-setrow__label' }, props.label, props.configKey && h('code', { className: 'lf-setrow__key' }, props.configKey)),
        props.description && h('div', { className: 'lf-setrow__desc' }, props.description)),
      h('div', { className: 'lf-setrow__control' }, props.children));
  }

  // keeps a draft while typing; Enter or blur commits, Esc reverts, validate() blocks a bad value with its message
  function SettingField(props) {
    var shown = props.value == null ? '' : String(props.value);
    var draft = useState(shown), err = useState(null), reveal = useState(false);
    useEffect(function () { draft[1](shown); err[1](null); }, [shown]);
    function commit() {
      var v = draft[0].trim();
      if (v === shown) { err[1](null); return; }
      var e = props.validate ? props.validate(v) : null;
      err[1](e);
      if (!e && props.onCommit) props.onCommit(props.type === 'number' ? (v === '' ? 0 : +v) : v);
    }
    var secret = props.type === 'password';
    return h('div', { className: 'lf-setfield', style: props.width ? { width: props.width + 'px' } : null },
      h('div', { className: 'lf-setfield__box' },
        h('input', { className: cx('lf-input', props.mono !== false && 'is-mono'), value: draft[0], disabled: props.disabled, placeholder: props.placeholder,
          type: secret && !reveal[0] ? 'password' : 'text', inputMode: props.type === 'number' ? 'numeric' : undefined, spellCheck: false, autoComplete: 'off',
          'aria-label': props['aria-label'], 'aria-invalid': err[0] ? true : undefined,
          onChange: function (e) { draft[1](e.target.value); if (err[0]) err[1](null); },
          onBlur: commit,
          onKeyDown: function (e) { if (e.key === 'Enter') { e.preventDefault(); commit(); } if (e.key === 'Escape') { draft[1](shown); err[1](null); } } }),
        props.suffix && h('span', { className: 'lf-setfield__suffix' }, props.suffix),
        secret && h(Button, { size: 'sm', variant: 'ghost', onClick: function () { reveal[1](!reveal[0]); } }, reveal[0] ? '隐藏' : '显示')),
      err[0] && h('div', { className: 'lf-field__error', role: 'alert' }, err[0]));
  }

  // one entry per line; validate(line) → message, reported with its line number
  function ListField(props) {
    var shown = (props.value || []).join('\n');
    var draft = useState(shown), err = useState(null);
    useEffect(function () { draft[1](shown); err[1](null); }, [shown]);
    function commit() {
      var lines = draft[0].split('\n').map(function (l) { return l.trim(); }).filter(Boolean);
      if (lines.join('\n') === shown) { draft[1](shown); err[1](null); return; }
      for (var i = 0; i < lines.length; i++) {
        var e = props.validate && props.validate(lines[i]);
        if (e) { err[1]('第 ' + (i + 1) + ' 行「' + lines[i] + '」：' + e); return; }
      }
      err[1](null); if (props.onCommit) props.onCommit(lines);
    }
    var rows = Math.max(props.minRows || 3, draft[0].split('\n').length);
    return h('div', { className: 'lf-setfield is-list' },
      h('textarea', { className: 'lf-input is-mono lf-listfield', rows: Math.min(rows, props.maxRows || 8), value: draft[0], disabled: props.disabled,
        placeholder: props.placeholder, spellCheck: false, 'aria-label': props['aria-label'], 'aria-invalid': err[0] ? true : undefined,
        onChange: function (e) { draft[1](e.target.value); if (err[0]) err[1](null); }, onBlur: commit }),
      err[0] ? h('div', { className: 'lf-field__error', role: 'alert' }, err[0]) : props.hint && h('div', { className: 'lf-field__desc' }, props.hint));
  }

  /* ---------- TextField + Select: labelled inputs ---------- */

  var uid = 0;
  function useUid(prefix) { var r = useRef(null); if (!r.current) r.current = prefix + (++uid); return r.current; }

  function TextField(props) {
    var id = useUid('lf-f'), note = props.error || props.description;
    var rest = Object.assign({}, props);
    ['label', 'description', 'error', 'mono', 'className', 'onValueChange'].forEach(function (k) { delete rest[k]; });
    return h('div', { className: cx('lf-field', props.error && 'is-invalid', props.className) },
      props.label && h('label', { className: 'lf-field__label', htmlFor: id }, props.label),
      h('input', Object.assign({ id: id, type: 'text', autoComplete: 'off', spellCheck: false }, rest, {
        className: cx('lf-input', props.mono && 'is-mono'),
        'aria-invalid': props.error ? true : undefined, 'aria-describedby': note ? id + '-n' : undefined,
        onChange: function (e) { if (props.onChange) props.onChange(e); if (props.onValueChange) props.onValueChange(e.target.value); }
      })),
      note && h('div', { id: id + '-n', className: props.error ? 'lf-field__error' : 'lf-field__desc', role: props.error ? 'alert' : undefined }, note));
  }

  // floating lists go to <body> so a transformed or clipping ancestor (Sheet, scroll panes) can't shift or cut them
  function portal(node) { return window.ReactDOM && ReactDOM.createPortal ? ReactDOM.createPortal(node, document.body) : node; }

  // a button + listbox drawn like ContextMenu, so the open list matches the system instead of the OS menu
  function Select(props) {
    var id = useUid('lf-s'), opts = props.options || [];
    var open = useState(false), hi = useState(0), at = useState(null);
    var btn = useRef(null), list = useRef(null), typed = useRef({ s: '', t: 0 });
    var idx = Math.max(0, opts.findIndex(function (o) { return o.value === props.value; }));
    var cur = opts[idx];
    function show() {
      if (props.disabled || !opts.length) return;
      var r = btn.current.getBoundingClientRect(), h = Math.min(opts.length * 30 + 10, 280), below = window.innerHeight - r.bottom - 8;
      at[1]({ left: r.left, width: r.width, top: below >= h || below > r.top ? r.bottom + 4 : r.top - h - 4 });
      hi[1](idx); open[1](true);
    }
    function hide(focus) { open[1](false); if (focus && btn.current) btn.current.focus(); }
    function pick(i) { var o = opts[i]; hide(true); if (o && o.value !== props.value && props.onChange) props.onChange(o.value); }
    useEffect(function () {
      if (!open[0]) return;
      function away(e) { if (!list.current.contains(e.target) && !btn.current.contains(e.target)) hide(false); }
      function shut(e) { if (!list.current || !list.current.contains(e.target)) hide(false); }
      document.addEventListener('pointerdown', away, true); window.addEventListener('resize', shut); document.addEventListener('scroll', shut, true); window.addEventListener('blur', shut);
      return function () { document.removeEventListener('pointerdown', away, true); window.removeEventListener('resize', shut); document.removeEventListener('scroll', shut, true); window.removeEventListener('blur', shut); };
    }, [open[0]]);
    useEffect(function () {
      var el = open[0] && list.current && list.current.children[hi[0]];
      if (el && el.scrollIntoView) el.scrollIntoView({ block: 'nearest' });
    }, [open[0], hi[0]]);
    function onKey(e) {
      var k = e.key, n = opts.length;
      if (!open[0]) {
        if (k === 'ArrowDown' || k === 'ArrowUp' || k === 'Enter' || k === ' ') { e.preventDefault(); show(); }
        return;
      }
      if (k === 'Escape' || k === 'Tab') { if (k === 'Escape') { e.preventDefault(); e.stopPropagation(); } hide(k === 'Escape'); return; }
      if (k === 'Enter' || k === ' ') { e.preventDefault(); pick(hi[0]); return; }
      var next = k === 'ArrowDown' ? Math.min(n - 1, hi[0] + 1) : k === 'ArrowUp' ? Math.max(0, hi[0] - 1) : k === 'Home' ? 0 : k === 'End' ? n - 1 : null;
      if (next != null) { e.preventDefault(); hi[1](next); return; }
      if (k.length === 1) {   // type to jump
        var now = Date.now(), t = typed.current; t.s = now - t.t > 600 ? k : t.s + k; t.t = now;
        var m = opts.findIndex(function (o) { return String(o.label || o.value).toLowerCase().indexOf(t.s.toLowerCase()) === 0; });
        if (m >= 0) hi[1](m);
      }
    }
    var box = h('span', { className: cx('lf-select', props.mono && 'is-mono', open[0] && 'is-open', !props.label && props.className) },
      h('button', { id: id, ref: btn, type: 'button', className: 'lf-select__btn', disabled: props.disabled, role: 'combobox',
        'aria-haspopup': 'listbox', 'aria-expanded': open[0], 'aria-controls': id + '-l', 'aria-label': props.label ? undefined : props['aria-label'],
        'aria-activedescendant': open[0] ? id + '-o' + hi[0] : undefined,
        onClick: function () { open[0] ? hide(false) : show(); }, onKeyDown: onKey },
        h('span', { className: 'lf-select__value' }, cur ? cur.label || cur.value : '')),
      h(Icon, { name: 'chevron', size: 14 }),
      open[0] && at[0] && portal(h('div', { id: id + '-l', ref: list, role: 'listbox', className: cx('lf-menu lf-select__list', props.mono && 'is-mono'), 'aria-labelledby': id,
        style: { left: at[0].left + 'px', top: at[0].top + 'px', minWidth: at[0].width + 'px' } },
        opts.map(function (o, i) {
          var sel = i === idx;
          return h('div', { key: o.value, id: id + '-o' + i, role: 'option', 'aria-selected': sel, className: cx('lf-menu__item lf-select__opt', i === hi[0] && 'is-hi', sel && 'is-selected'),
            onPointerEnter: function () { hi[1](i); }, onPointerDown: function (e) { e.preventDefault(); }, onClick: function () { pick(i); } },
            h('span', { className: 'lf-select__check', 'aria-hidden': true }, sel && h(Icon, { name: 'check', size: 14 })), h('span', null, o.label || o.value));
        }))));
    if (!props.label) return box;
    return h('div', { className: cx('lf-field', props.className) },
      h('label', { className: 'lf-field__label', htmlFor: id }, props.label), box,
      props.description && h('div', { className: 'lf-field__desc' }, props.description));
  }

  /* ---------- CodeEditor: YAML with line numbers and an error mark ---------- */

  var LINE = 20;

  function CodeEditor(props) {
    var value = props.value || '', count = value.split('\n').length, side = useRef(null), mark = useRef(null);
    var nums = [];
    for (var i = 1; i <= count; i++) nums.push(h('span', { key: i, className: i === props.errorLine ? 'is-error' : undefined }, i));
    function sync(top) {
      if (side.current) side.current.scrollTop = top;
      if (mark.current) mark.current.style.transform = 'translateY(' + ((props.errorLine - 1) * LINE - top) + 'px)';
    }
    return h('div', { className: cx('lf-code', props.className), style: { height: props.height } },
      h('div', { className: 'lf-code__gutter', ref: side, 'aria-hidden': true }, nums),
      h('div', { className: 'lf-code__area' },
        props.errorLine && h('div', { className: 'lf-code__mark', ref: mark, style: { transform: 'translateY(' + (props.errorLine - 1) * LINE + 'px)' }, 'aria-hidden': true }),
        h('textarea', { className: 'lf-code__text', value: value, readOnly: props.readOnly, wrap: 'off',
          spellCheck: false, autoCapitalize: 'off', autoComplete: 'off', 'aria-label': props['aria-label'] || '配置文件',
          onScroll: function (e) { sync(e.target.scrollTop); },
          onChange: function (e) { if (props.onChange) props.onChange(e.target.value); } })));
  }

  // mihomo's own error text is shown as-is; we only lift a line number out of it to mark the editor
  function errorLineOf(message) { var m = /\bline (\d+)/i.exec(message || ''); return m ? +m[1] : null; }

  /* ---------- Sheet: a side panel that glides in for editing ---------- */

  function Sheet(props) {
    var mounted = useState(!!props.open), shown = useState(false), panel = useRef(null), back = useRef(null);
    var close = useRef(props.onClose); close.current = props.onClose;
    useEffect(function () {
      if (props.open) {
        back.current = document.activeElement; mounted[1](true);
        var r = requestAnimationFrame(function () { r = requestAnimationFrame(function () { shown[1](true); }); });
        return function () { cancelAnimationFrame(r); };
      }
      shown[1](false);
      var t = setTimeout(function () { mounted[1](false); if (back.current && back.current.focus) back.current.focus(); }, reducedMotion() ? 0 : 420);
      return function () { clearTimeout(t); };
    }, [props.open]);
    useEffect(function () {
      if (!shown[0]) return;
      var f = panel.current && panel.current.querySelector('.lf-sheet__body input, .lf-sheet__body textarea, .lf-sheet__body select');
      if (f) f.focus();
      function key(e) { if (e.key === 'Escape' && close.current) close.current(); }
      window.addEventListener('keydown', key);
      return function () { window.removeEventListener('keydown', key); };
    }, [shown[0]]);
    if (!mounted[0]) return null;
    return h('div', { className: cx('lf-sheet', shown[0] && 'is-open', props.className) },
      h('div', { className: 'lf-sheet__scrim', onClick: props.onClose }),
      h('div', { className: 'lf-sheet__panel', role: 'dialog', 'aria-modal': true, 'aria-label': props.title, ref: panel, style: { width: props.width } },
        h('div', { className: 'lf-sheet__head' },
          h('div', { className: 'lf-sheet__titles' },
            props.kicker && h('div', { className: 'lf-sheet__kicker' }, props.kicker),
            h('h2', { className: 'lf-sheet__title' }, props.title)),
          h(Button, { variant: 'ghost', size: 'sm', icon: 'close', 'aria-label': '关闭', onClick: props.onClose })),
        props.tabs && h('div', { className: 'lf-sheet__tabs' }, props.tabs),
        h('div', { className: 'lf-sheet__body' }, props.children),
        props.footer && h('div', { className: 'lf-sheet__foot' }, props.footer)));
  }

  /* ---------- ProfileEditor: name, subscription URL, update interval, and the YAML itself ---------- */

  var UPDATE_EVERY = [{ value: '0', label: '不自动更新' }, { value: '6', label: '每 6 小时' }, { value: '12', label: '每 12 小时' }, { value: '24', label: '每天' }];
  var BLANK_PROFILE = { name: '', source: 'url', url: '', interval: '24', content: '' };

  function ProfileEditor(props) {
    var p = props.profile || {}, creating = !p.id;
    var d = useState(Object.assign({}, BLANK_PROFILE, p)), draft = d[0];
    var tab = useState('info'), tried = useState(false);
    // check: the result of `mihomo -t` on this draft — idle | running | fail (with mihomo's message)
    var ck = useState({ state: 'idle' }), check = ck[0];
    useEffect(function () { if (props.open) { d[1](Object.assign({}, BLANK_PROFILE, props.profile || {})); tab[1](props.defaultTab || (p.error ? 'file' : 'info')); tried[1](false); ck[1]({ state: 'idle' }); } }, [props.open]);
    function set(k, v) { if (working) return; d[1](function (x) { var n = Object.assign({}, x); n[k] = v; return n; }); if (k === 'content') ck[1]({ state: 'idle' }); }

    var isUrl = draft.source === 'url', hasFile = !(creating && isUrl);
    var errs = {
      name: !draft.name.trim() && '给这份配置起个名字',
      url: isUrl && !/^https?:\/\/\S+$/.test(draft.url.trim()) && '链接应以 http:// 或 https:// 开头'
    };
    var changed = draft.content !== (p.content || '');
    // what mihomo said: this save's check, or — until the file is touched — the error the profile already carries
    var fail = check.state === 'fail' ? check.message : (!changed && check.state === 'idle' && p.error) || null;
    var working = check.state === 'running' || !!props.busy;
    function failed(error) { ck[1]({ state: 'fail', message: String(error) }); tab[1](hasFile ? 'file' : 'info'); }
    function done() {
      ck[1]({ state: 'running' });
      return Promise.resolve().then(function () { return props.onSave(Object.assign({}, draft, { name: draft.name.trim(), url: draft.url.trim() })); })
        .then(function () { ck[1]({ state: 'idle' }); }).catch(failed);
    }
    function save() {
      if (working) return;
      if (errs.name || errs.url) { tried[1](true); tab[1]('info'); return; }
      if (!hasFile || !props.validate) { done(); return; }
      ck[1]({ state: 'running' });
      Promise.resolve().then(function () { return props.validate(draft.content); }).then(function (message) {
        if (message) { ck[1]({ state: 'fail', message: String(message) }); tab[1]('file'); } else done();
      }).catch(failed);
    }
    var saveLabel = creating ? (isUrl ? '添加并下载' : '添加') : isUrl && draft.url.trim() !== (p.url || '').trim() ? '保存并下载' : (props.active && props.running !== false && changed ? '保存并重新加载' : '保存');

    var info = h('div', { className: 'lf-editor__form' },
      creating && h('div', { className: 'lf-field' },
        h('span', { className: 'lf-field__label' }, '来源'),
        h(SegmentedControl, { 'aria-label': '来源', value: draft.source, onChange: function (v) { set('source', v); },
          options: [{ value: 'url', label: '订阅链接', icon: 'globe' }, { value: 'file', label: '本地文件', icon: 'layers' }] })),
      h(TextField, { label: '名称', value: draft.name, placeholder: '例如：机场 A', onValueChange: function (v) { set('name', v); }, error: tried[0] && errs.name }),
      isUrl && h(TextField, { label: '订阅链接', mono: true, value: draft.url, placeholder: 'https://…', onValueChange: function (v) { set('url', v); },
        error: tried[0] && errs.url, description: '在机场后台复制「Clash 订阅」链接粘贴到这里' }),
      isUrl && h(Select, { label: '自动更新', value: draft.interval, options: UPDATE_EVERY, onChange: function (v) { set('interval', v); },
        description: '应用打开时按间隔更新；失败会保留旧文件' }),
      !isUrl && h('div', { className: 'lf-field' },
        h('span', { className: 'lf-field__label' }, '文件'),
        h('div', { className: 'lf-editor__file' },
          h('span', { className: 'lf-editor__path' }, draft.path || (draft.content ? '应用内保存的配置' : '还没有选择文件')),
          h(Button, { size: 'sm', icon: 'layers', onClick: function () { if (props.onPickFile) Promise.resolve().then(function () { return props.onPickFile(function (f) { d[1](function (x) { return Object.assign({}, x, f, { name: x.name || f.name }); }); ck[1]({ state: 'idle' }); }); }).catch(failed); } }, '选择 .yaml'))),
      !isUrl && h('p', { className: 'lf-field__desc' }, '导入后保存为应用内副本，不会改写原文件。也可以切到「配置文件」直接粘贴内容。'));

    var file = h('div', { className: 'lf-editor__filetab' },
      !creating && isUrl && h('div', { className: 'lf-notice' }, h(Icon, { name: 'alert-circle', size: 14 }),
        h('span', null, '这是订阅下载的文件，下次更新会覆盖手动修改。想长期生效的规则请放进「覆盖规则」。'),
        props.onGoOverrides && h(Button, { variant: 'ghost', size: 'sm', iconAfter: 'arrow-right', onClick: function () { if (!changed || window.confirm('配置文件尚未保存，仍前往覆盖规则？')) props.onGoOverrides(); } }, '覆盖规则')),
      h(CodeEditor, { value: draft.content, onChange: function (v) { set('content', v); }, errorLine: errorLineOf(fail), 'aria-label': draft.name + ' 配置文件' }),
      h('div', { className: cx('lf-editor__status', fail ? 'is-fail' : 'is-idle'), role: 'status' },
        h(Icon, { name: fail ? 'close' : check.state === 'running' ? 'refresh' : 'check', size: 12, className: check.state === 'running' ? 'is-spin' : null }),
        h('span', null, fail ? String(fail) : check.state === 'running' ? '正在用 mihomo 校验…' : '保存时用 mihomo 校验，有错会显示它的原话'),
        changed && !creating && h(Button, { variant: 'ghost', size: 'sm', icon: 'refresh', onClick: function () { set('content', p.content || ''); } }, '撤销修改')));

    return h(Sheet, {
      open: props.open, onClose: function () { if (!working) props.onClose(); }, width: props.width || 560,
      kicker: creating ? '添加配置' : (isUrl ? '订阅' : '本地文件'), title: creating ? (draft.name || '新配置') : p.name,
      tabs: hasFile && h(SegmentedControl, { size: 'sm', 'aria-label': '编辑内容', value: tab[0], onChange: tab[1],
        options: [{ value: 'info', label: '基本信息' }, { value: 'file', label: '配置文件' + (fail ? ' · 有错误' : '') }] }),
      footer: [
        !creating && props.onDelete && h(Button, { key: 'del', variant: 'ghost', size: 'sm', disabled: props.active || working, title: props.active ? '正在使用的配置不能删除' : undefined, onClick: function () { Promise.resolve().then(props.onDelete).catch(failed); } }, '删除'),
        h('span', { key: 'sp', className: 'lf-sheet__spacer' }),
        h(Button, { key: 'no', disabled: working, onClick: props.onClose }, '取消'),
        h(Button, { key: 'ok', variant: 'primary', disabled: working, onClick: save }, working ? '处理中…' : saveLabel)]
    }, h('fieldset', { disabled: working, className: 'lf-editor__fieldset' }, tab[0] === 'file' && hasFile ? file : info,
      fail && !(tab[0] === 'file' && hasFile) && h('p', { className: 'lf-field__error', role: 'alert' }, String(fail))));
  }

  /* ---------- OverrideGroup: a named set of the user's own rules that sits before the subscription's ---------- */

  var RULE_TYPES = [
    { value: 'DOMAIN-SUFFIX', label: '域名及子域名', hint: 'corp.example.com' },
    { value: 'DOMAIN', label: '完整域名', hint: 'wiki.corp.example.com' },
    { value: 'DOMAIN-KEYWORD', label: '域名关键词', hint: 'intranet' },
    { value: 'IP-CIDR', label: 'IP 网段', hint: '10.0.0.0/8' },
    { value: 'PROCESS-NAME', label: '应用', hint: 'Feishu' }
  ];
  var RULE_LABEL = { 'IP-CIDR6': 'IP 网段' };
  RULE_TYPES.forEach(function (t) { RULE_LABEL[t.value] = t.label; });
  var POLICY_LABEL = { DIRECT: '直连', REJECT: '拦截' };

  // Forgive what people paste: a full URL becomes its host, a bare IP becomes /32.
  function normalizeRule(type, raw) {
    var v = (raw || '').trim();
    if (!v) return { error: '先填写内容' };
    if (type.indexOf('DOMAIN') === 0) {
      v = v.replace(/^[a-z][a-z0-9+.-]*:\/\//i, '').replace(/[\/?#:].*$/, '').replace(/^\*?\./, '').toLowerCase();
      if (!/^[a-z0-9¡-￿-]+(\.[a-z0-9¡-￿-]+)*$/.test(v)) return { error: '这不像一个域名' };
      return { type: type, payload: v };
    }
    if (type === 'IP-CIDR') {
      if (v.indexOf(':') >= 0) return /^[0-9a-f:]+(\/\d{1,3})?$/i.test(v) ? { type: 'IP-CIDR6', payload: v.indexOf('/') < 0 ? v + '/128' : v } : { error: '这不像一个 IP 网段' };
      if (v.indexOf('/') < 0) v += '/32';
      var m = v.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\/(\d{1,2})$/);
      if (!m || m.slice(1, 5).some(function (x) { return +x > 255; }) || +m[5] > 32) return { error: '格式应为 10.0.0.0/8 或单个 IP' };
      return { type: type, payload: v };
    }
    return { type: type, payload: v };
  }

  function OverrideGroup(props) {
    var rules = props.rules || [];
    var open = useState(!!props.defaultOpen), renaming = useState(false), name = useState(props.name);
    var ty = useState('DOMAIN-SUFFIX'), val = useState(''), pol = useState('DIRECT'), err = useState(null);
    var policies = ['DIRECT', 'REJECT'].concat(props.policies || []);
    var hint = RULE_TYPES.filter(function (t) { return t.value === ty[0]; })[0].hint;
    function add() {
      var r = normalizeRule(ty[0], val[0]);
      if (r.error) { err[1](r.error); return; }
      if (rules.some(function (x) { return x.type === r.type && x.payload === r.payload; })) { err[1]('这条规则已经在分组里了'); return; }
      props.onChange(rules.concat([{ type: r.type, payload: r.payload, policy: pol[0] }]));
      val[1](''); err[1](null);
    }
    function rename() { renaming[1](false); var n = name[0].trim(); if (n && n !== props.name && props.onRename) props.onRename(n); else name[1](props.name); }
    var bodyId = useUid('lf-ovr');
    var menu = useState(null);
    // Rename, reorder and delete are rare, so they live behind one ⋯ menu instead of loose buttons around the card.
    var items = [
      props.onRename && { label: '重命名', icon: 'edit', onSelect: function () { name[1](props.name); open[1](true); renaming[1](true); } },
      (props.onMoveUp !== undefined || props.onMoveDown !== undefined) && { separator: true },
      props.onMoveUp !== undefined && { label: '上移', icon: 'arrow-up', disabled: !props.onMoveUp, onSelect: props.onMoveUp || undefined },
      props.onMoveDown !== undefined && { label: '下移', icon: 'arrow-down', disabled: !props.onMoveDown, onSelect: props.onMoveDown || undefined },
      props.onDelete && { separator: true },
      props.onDelete && { label: '删除分组', icon: 'close', onSelect: props.onDelete }
    ].filter(Boolean);
    if (items.length && items[0].separator) items.shift();
    return h('article', { className: cx('lf-ovr', props.enabled && 'is-on', open[0] && 'is-open', props.className) },
      h('div', { className: 'lf-ovr__head' },
        renaming[0]
          ? h('input', { className: 'lf-input lf-ovr__rename', value: name[0], autoFocus: true, 'aria-label': '分组名称',
              onChange: function (e) { name[1](e.target.value); }, onBlur: rename,
              onKeyDown: function (e) { if (e.key === 'Enter') rename(); if (e.key === 'Escape') { name[1](props.name); renaming[1](false); } } })
          : h('button', { type: 'button', className: 'lf-ovr__toggle', 'aria-expanded': open[0], 'aria-controls': bodyId, onClick: function () { open[1](!open[0]); } },
              h(Icon, { name: 'chevron', className: 'lf-ovr__chev' }),
              h('span', { className: 'lf-ovr__name' }, props.name),
              h('span', { className: 'lf-ovr__count' }, rules.length + ' 条')),
        h('span', { className: 'lf-ovr__state' }, h('i', { 'aria-hidden': true }), props.enabled ? '生效中' : '未启用'),
        items.length > 0 && h(Button, { variant: 'ghost', size: 'sm', icon: 'more', 'aria-label': '分组操作：' + props.name, 'aria-haspopup': 'menu',
          onClick: function (e) { var r = e.currentTarget.getBoundingClientRect(); menu[1]({ x: r.right - 180, y: r.bottom + 4 }); } }),
        h(Switch, { checked: !!props.enabled, onChange: props.onToggle, 'aria-label': '启用「' + props.name + '」' })),
      menu[0] && h(ContextMenu, { open: true, x: menu[0].x, y: menu[0].y, items: items, label: '分组操作', onClose: function () { menu[1](null); } }),
      open[0] && h('div', { className: 'lf-ovr__body', id: bodyId },
        rules.length ? h('ol', { className: 'lf-ovr__rules' }, rules.map(function (r, i) {
          return h('li', { key: r.type + r.payload, className: 'lf-ovr__rule' },
            h('span', { className: 'lf-ovr__type', title: r.type }, RULE_LABEL[r.type] || r.type),
            h('span', { className: 'lf-ovr__payload' }, r.payload),
            h('span', { className: 'lf-ovr__policy' }, POLICY_LABEL[r.policy] || r.policy),
            h(Button, { variant: 'ghost', size: 'sm', icon: 'close', 'aria-label': '删除 ' + r.payload,
              onClick: function () { props.onChange(rules.filter(function (_, j) { return j !== i; })); } }));
        })) : h('p', { className: 'lf-ovr__empty' }, '还没有规则，用下面这一行添加第一条。'),
        h('div', { className: 'lf-ovr__add' },
          h(Select, { 'aria-label': '规则类型', value: ty[0], onChange: function (v) { ty[1](v); err[1](null); },
            options: RULE_TYPES.map(function (t) { return { value: t.value, label: t.label }; }) }),
          h('input', { className: cx('lf-input is-mono', err[0] && 'is-invalid'), value: val[0], placeholder: hint, 'aria-label': '规则内容', 'aria-invalid': err[0] ? true : undefined,
            onChange: function (e) { val[1](e.target.value); err[1](null); }, onKeyDown: function (e) { if (e.key === 'Enter') add(); } }),
          h(Select, { 'aria-label': '走向', value: pol[0], onChange: pol[1], options: policies.map(function (p) { return { value: p, label: POLICY_LABEL[p] ? POLICY_LABEL[p] + ' ' + p : p }; }) }),
          h(Button, { size: 'sm', icon: 'plus', disabled: !val[0].trim(), onClick: add }, '添加')),
        err[0] && h('div', { className: 'lf-field__error', role: 'alert' }, err[0])));
  }

  /* ---------- RuleGuide: how override rules work, in one card ---------- */

  var GUIDE_TYPES = [
    ['域名及子域名', 'corp.com', '匹配它和所有子域名（wiki.corp.com…），最常用'],
    ['完整域名', 'wiki.corp.com', '只匹配这一个地址'],
    ['域名关键词', 'intranet', '域名里含这个词就算，范围大，慎用'],
    ['IP 网段', '10.0.0.0/8', '内网地址、NAS；单个 IP 直接填'],
    ['应用', 'Feishu', '这个应用的全部流量（填进程名）']
  ];
  var GUIDE_POLICIES = [['直连', '不走代理：内网、国内网站'], ['拦截', '直接拒绝：广告、追踪'], ['策略组', '走代理：用「代理」页选的节点']];
  var GUIDE_STEPS = [
    ['新建分组', '按场景起名，比如「公司内网」。'],
    ['添加规则', '选类型 → 填网址或 IP → 选走向。粘贴整条网址也可以。'],
    ['打开开关', '立即生效。覆盖规则排在订阅规则前面，从上到下先匹配先生效。']
  ];

  function RuleGuide(props) {
    return h('section', { className: cx('lf-guide', props.className), 'aria-label': '覆盖规则使用指导' },
      h('div', { className: 'lf-guide__head' },
        h('div', { className: 'lf-guide__text' },
          h('h2', { className: 'lf-guide__title' }, props.title || '覆盖规则怎么用'),
          h('p', { className: 'lf-guide__lede' }, props.description || '让某些网站固定直连、走代理或被拦截。公司内网、NAS 打不开，多半是被送进了代理——给它加一条「直连」。')),
        props.onDismiss && h(Button, { variant: 'ghost', size: 'sm', icon: 'close', 'aria-label': '收起使用指导', onClick: props.onDismiss })),
      h('ol', { className: 'lf-guide__steps' }, GUIDE_STEPS.map(function (s, i) {
        return h('li', { key: i }, h('span', { className: 'lf-guide__num' }, i + 1), h('span', null, h('b', null, s[0]), s[1]));
      })),
      h('div', { className: 'lf-guide__cols' },
        h('div', null,
          h('div', { className: 'lf-guide__cap' }, '规则类型'),
          h('dl', { className: 'lf-guide__types' }, GUIDE_TYPES.map(function (t) {
            return [h('dt', { key: t[0] + 'k' }, t[0]), h('dd', { key: t[0] + 'v' }, h('code', null, t[1]), h('span', null, t[2]))];
          }))),
        h('div', null,
          h('div', { className: 'lf-guide__cap' }, '走向'),
          h('dl', { className: 'lf-guide__types' }, GUIDE_POLICIES.map(function (t) {
            return [h('dt', { key: t[0] + 'k' }, t[0]), h('dd', { key: t[0] + 'v' }, h('span', null, t[1]))];
          })),
          h('p', { className: 'lf-guide__tip' }, '不确定某个网站走了哪条规则？去「连接」页看它的「规则」一栏。'))),
      (props.onStartTour || props.onTemplate) && h('div', { className: 'lf-guide__foot' },
        props.onStartTour && h(Button, { size: 'sm', variant: 'primary', icon: 'arrow-right', onClick: props.onStartTour }, '跟着做一遍'),
        props.onTemplate && h(Button, { size: 'sm', variant: 'ghost', icon: 'plus', onClick: props.onTemplate }, '添加「局域网直连」模板')));
  }

  /* ---------- ContextMenu: right-click actions at the pointer ---------- */

  function ContextMenu(props) {
    var ref = useRef(null), pos = useState(null);
    var close = useRef(props.onClose); close.current = props.onClose;
    var items = props.items || [];
    useLayoutEffect(function () {
      if (!props.open || !ref.current) { pos[1](null); return; }
      var r = ref.current.getBoundingClientRect(), vw = window.innerWidth, vh = window.innerHeight;
      pos[1]({ x: clamp(props.x, 8, vw - r.width - 8), y: props.y + r.height > vh - 8 ? Math.max(8, props.y - r.height) : props.y });
    }, [props.open, props.x, props.y]);
    // focus once it's measured and visible; a visibility:hidden button can't take focus
    useEffect(function () {
      if (!pos[0] || !ref.current) return;
      var first = ref.current.querySelector('.lf-menu__item:not([disabled])'); if (first) first.focus();
    }, [pos[0]]);
    useEffect(function () {
      if (!props.open) return;
      function away(e) { if (ref.current && !ref.current.contains(e.target) && close.current) close.current(); }
      function shut() { if (close.current) close.current(); }
      document.addEventListener('pointerdown', away, true);
      window.addEventListener('resize', shut); window.addEventListener('blur', shut); document.addEventListener('scroll', shut, true);
      return function () {
        document.removeEventListener('pointerdown', away, true);
        window.removeEventListener('resize', shut); window.removeEventListener('blur', shut); document.removeEventListener('scroll', shut, true);
      };
    }, [props.open]);
    if (!props.open) return null;
    function onKey(e) {
      var list = Array.prototype.slice.call(ref.current.querySelectorAll('.lf-menu__item:not([disabled])'));
      var i = list.indexOf(document.activeElement);
      if (e.key === 'Escape' || e.key === 'Tab') { e.preventDefault(); close.current(); return; }
      var n = e.key === 'ArrowDown' ? i + 1 : e.key === 'ArrowUp' ? i - 1 : e.key === 'Home' ? 0 : e.key === 'End' ? list.length - 1 : null;
      if (n == null) return;
      e.preventDefault(); list[(n + list.length) % list.length].focus();
    }
    return h('div', { className: 'lf-menu', role: 'menu', 'aria-label': props.label, ref: ref, onKeyDown: onKey,
      style: { left: (pos[0] ? pos[0].x : props.x) + 'px', top: (pos[0] ? pos[0].y : props.y) + 'px', visibility: pos[0] ? 'visible' : 'hidden' } },
      props.title && h('div', { className: 'lf-menu__title' }, props.title),
      items.map(function (it, i) {
        if (it.separator) return h('div', { key: i, className: 'lf-menu__sep', role: 'separator' });
        return h('button', { key: i, type: 'button', role: it.selected === undefined ? 'menuitem' : 'menuitemradio', 'aria-checked': it.selected, disabled: it.disabled, className: cx('lf-menu__item', it.primary && 'is-primary'),
          onClick: function () { close.current(); if (it.onSelect) it.onSelect(); } },
          h(Icon, { name: it.icon || 'arrow-right', size: 14 }), h('span', null, it.label), it.hint && h('kbd', null, it.hint), it.selected && h(Icon, { name: 'check', size: 14 }));
      }));
  }

  /* ---------- AddRuleSheet: turn one connection into an override rule ---------- */

  function isIpHost(host) { return /^\d{1,3}(\.\d{1,3}){3}$/.test(host || '') || (host || '').indexOf(':') >= 0; }
  // a.b.example.com → example.com; foo.example.co.uk → example.co.uk
  function siteOf(host) {
    var parts = (host || '').toLowerCase().split('.');
    if (parts.length <= 2) return parts.join('.');
    var n = parts[parts.length - 1].length === 2 && /^(com|net|org|gov|edu|co|ac|or|ne|go)$/.test(parts[parts.length - 2]) ? 3 : 2;
    return parts.slice(-n).join('.');
  }
  function ruleChoicesFor(c) {
    var out = [], host = c.host || '';
    if (isIpHost(host)) {
      if (host.indexOf(':') >= 0) out.push({ type: 'IP-CIDR6', payload: host + '/128', label: '这个 IP', desc: '只匹配这一个地址' });
      else {
        out.push({ type: 'IP-CIDR', payload: host + '/32', label: '这个 IP', desc: '只匹配这一个地址' });
        out.push({ type: 'IP-CIDR', payload: host.split('.').slice(0, 3).join('.') + '.0/24', label: '同一网段', desc: '同一 /24 网段的所有地址' });
      }
    } else if (host) {
      var site = siteOf(host);
      out.push({ type: 'DOMAIN-SUFFIX', payload: site, label: '域名及子域名', desc: '也匹配 *.' + site + '，最常用' });
      if (site !== host) out.push({ type: 'DOMAIN', payload: host, label: '完整域名', desc: '只匹配这一个地址' });
    }
    if (c.process) out.push({ type: 'PROCESS-NAME', payload: c.process, label: '应用', desc: c.process + ' 的全部流量' });
    return out;
  }
  var NEW_GROUP = '__new__';

  function AddRuleSheet(props) {
    var c = props.connection || {}, groups = props.groups || [], choices = ruleChoicesFor(c);
    var policies = ['DIRECT', 'REJECT'].concat(props.policies || []);
    var direct = (c.chain || []).slice(-1)[0] === 'DIRECT';
    var pick = useState(0), pol = useState('DIRECT'), grp = useState(''), name = useState('');
    // opposite of what it does now: proxied traffic defaults to 直连, direct traffic to the first policy group
    useEffect(function () {
      if (!props.open) return;
      pick[1](0); pol[1](direct ? (props.policies || [])[0] || 'DIRECT' : 'DIRECT');
      grp[1](props.defaultGroupId || (groups[0] && groups[0].id) || NEW_GROUP); name[1]('');
    }, [props.open, c.id]);
    var choice = choices[pick[0]] || choices[0];
    var g = groups.filter(function (x) { return x.id === grp[0]; })[0];
    var dup = g && choice && g.rules.some(function (r) { return r.type === choice.type && r.payload === choice.payload; });
    var bad = !choice || dup || (grp[0] === NEW_GROUP && !name[0].trim());
    function add() { if (!bad) props.onAdd({ rule: { type: choice.type, payload: choice.payload, policy: pol[0] }, groupId: grp[0] === NEW_GROUP ? null : grp[0], newGroupName: name[0].trim() }); }
    return h(Sheet, { open: props.open, onClose: props.onClose, width: props.width || 440, kicker: '添加规则', title: c.host || '—',
      footer: [h('span', { key: 's', className: 'lf-sheet__spacer' }), h(Button, { key: 'c', onClick: props.onClose }, '取消'), h(Button, { key: 'o', variant: 'primary', disabled: bad, onClick: add }, '添加规则')] },
      h('div', { className: 'lf-editor__form' },
        h('div', { className: 'lf-quick__now' },
          h('span', null, '现在'), h('code', null, c.rule || '—'), h('span', null, '→'), h('code', null, ['你'].concat(c.chain || []).join(' → '))),
        h('fieldset', { className: 'lf-choice' },
          h('legend', { className: 'lf-field__label' }, '匹配方式'),
          choices.map(function (o, i) {
            return h('label', { key: o.type + o.payload, className: cx('lf-choice__item', i === pick[0] && 'is-on') },
              h('input', { type: 'radio', name: 'lf-quick-match', checked: i === pick[0], onChange: function () { pick[1](i); } }),
              h('span', { className: 'lf-choice__text' }, h('span', null, o.label, ' ', h('code', null, o.payload)), h('span', { className: 'lf-choice__desc' }, o.desc)));
          })),
        h(Select, { label: '走向', value: pol[0], onChange: pol[1], options: policies.map(function (p) { return { value: p, label: POLICY_LABEL[p] ? POLICY_LABEL[p] + ' ' + p : p }; }) }),
        h(Select, { label: '加到分组', value: grp[0], onChange: grp[1],
          options: groups.map(function (x) { return { value: x.id, label: x.name + (x.enabled ? '' : '（未启用）') }; }).concat([{ value: NEW_GROUP, label: '新建分组…' }]),
          description: g && !g.enabled ? '这个分组还没启用，添加后会一起打开。' : '规则排在订阅规则前面，对新连接生效。' }),
        grp[0] === NEW_GROUP && h(TextField, { label: '新分组名称', value: name[0], onValueChange: name[1], placeholder: '例如：公司内网' }),
        dup && h('div', { className: 'lf-field__error', role: 'alert' }, '「' + g.name + '」里已经有这条规则了')));
  }

  window.RocketScience = Object.assign(window.RocketScience || {}, {
    ConnectivityCheck: ConnectivityCheck, RuleGuide: RuleGuide, ContextMenu: ContextMenu, AddRuleSheet: AddRuleSheet, CheckPopover: CheckPopover, SettingsSection: SettingsSection, SettingRow: SettingRow, SettingField: SettingField, ListField: ListField, ruleChoicesFor: ruleChoicesFor, takeoverNote: takeoverNote, takeoverSummary: takeoverSummary,
    TextField: TextField, Select: Select, CodeEditor: CodeEditor, Sheet: Sheet, ProfileEditor: ProfileEditor, OverrideGroup: OverrideGroup,
    errorLineOf: errorLineOf, normalizeRule: normalizeRule, RULE_TYPES: RULE_TYPES,
    TrayIndicator: TrayIndicator, MenuBarPopover: MenuBarPopover, Tour: Tour, FirstRunCard: FirstRunCard, Flag: Flag, flagOf: flagOf, SortMenu: SortMenu, sortNodes: sortNodes, sortByName: sortByName, compareNames: compareNames,
    Button: Button, Switch: Switch, SegmentedControl: SegmentedControl, IgnitionButton: IgnitionButton,
    TelemetryStat: TelemetryStat, TelemetryHistory: TelemetryHistory, geoOf: geoOf, LatencyBadge: LatencyBadge, ProxyNode: ProxyNode, ProxyGroup: ProxyGroup,
    Trajectory: Trajectory, ConnectionTable: ConnectionTable, LogStream: LogStream, ProfileCard: ProfileCard,
    SearchField: SearchField, Sidebar: Sidebar, Icon: Icon,
    springs: springs, Body: Body, useBody: useBody, project: project, handoff: handoff, fmt: fmt
  });
})();
