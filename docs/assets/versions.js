const targetNode = document.querySelector('.md-header__inner');
const observerOptions = {
  childList: true,
  subtree: true
};

const observerCallback = function(mutationsList, observer) {
  for (let mutation of mutationsList) {
    if (mutation.type === 'childList') {
      const titleElement = document.querySelector('.md-header__inner > .md-header__title');
      if (titleElement) {
        initializeVersionDropdown();
        observer.disconnect();
      }
    }
  }
};

const observer = new MutationObserver(observerCallback);
observer.observe(targetNode, observerOptions);

function initializeVersionDropdown() {
  const callbackName = 'callback_' + new Date().getTime();
  window[callbackName] = function(response) {
    const div = document.createElement('div');
    div.innerHTML = response.html;
    document.querySelector(".md-header__inner > .md-header__title").appendChild(div);
    const container = div.querySelector('.rst-versions');
    var caret = document.createElement('div');
    caret.innerHTML = "<i class='fa fa-caret-down dropdown-caret'></i>";
    caret.classList.add('dropdown-caret');
    div.querySelector('.rst-current-version').appendChild(caret);

    div.querySelector('.rst-current-version').addEventListener('click', function() {
      container.classList.toggle('shift-up');
    });
  };

  var CSSLink = document.createElement('link');
  CSSLink.rel='stylesheet';
  CSSLink.href = '/assets/versions.css';
  document.getElementsByTagName('head')[0].appendChild(CSSLink);

  var script = document.createElement('script');
  script.src = 'https://rollouts-plugin-trafficrouter-gatewayapi.readthedocs.io/_/api/v2/footer_html/?'+
      'callback=' + callbackName + '&project=rollouts-plugin-trafficrouter-gatewayapi&page=&theme=mkdocs&format=jsonp&docroot=docs&source_suffix=.md&version=' + (window['READTHEDOCS_DATA'] || { version: 'latest' }).version;
  document.getElementsByTagName('head')[0].appendChild(script);
}