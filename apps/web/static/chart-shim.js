(function () {
    if (window.$) return;
    window.$ = {
        ajax: function (options) {
            const method = options.type || options.method || 'GET';
            const init = { method };
            if (method.toUpperCase() !== 'GET' && options.data) {
                init.body = typeof options.data === 'string' ? options.data : JSON.stringify(options.data);
            }
            fetch(options.url, init)
                .then(resp => options.dataType === 'json' ? resp.json() : resp.text())
                .then(data => options.success && options.success(data))
                .catch(error => options.error && options.error(error));
        }
    };
})();
