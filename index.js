// hide the page until fully setup
document.documentElement.style.setProperty('opacity', '0')

let loading = load([
  'https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github-dark.min.css',
  'https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/styles/github.min.css',
  'https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/highlight.min.js',
  'https://cdnjs.cloudflare.com/ajax/libs/highlight.js/11.9.0/languages/go.min.js',
  '../index.css?' + document.querySelector('script[src*="index.js"]').src.split('?').pop(),
])

// wait for the page to fully load
document.addEventListener('DOMContentLoaded', main)

function main() {
  // wait for highlight.js to load
  if (!window.hljs) {
    console.log('loading: waiting for highlight.js to load...')
    setTimeout(main, 100)
    return
  }

  // wait for all assets to load
  if (!loading.isDone()) {
    console.log('loading: waiting for assets to load...')
    setTimeout(main, 100)
    return
  }

  // setup the layout
  configureLegend()
  configureFileSelect()
  addIncrementalButton()
  addThemeButton()

  // setup the content
  configureCodeBlocks()
  configureSyntaxHighlight('pre .code .editor')
  addCoverageSpans('pre .coverage .editor')
  addLineNumbers()

  // setup complete, restore the page visibility
  document.documentElement.style.setProperty('opacity', '1')
}

function configureLegend() {
  let spans = document.querySelectorAll('#legend span')
  for (let i = 0; i < spans.length; i++) {
    if (spans[i].classList.length === 0) continue

    if (spans[i].classList[0] === 'cov0') {
      spans[i].classList.add('cov-uncovered')
    } else {
      spans[i].classList.add('cov-covered')
    }
  }
}

function addCoverageSpans(cssSelector) {
  let spans = Array.from(document.querySelectorAll(`${cssSelector} span`))

  spans.forEach((span) => {
    let html = span.innerHTML
    let lines = html.split('\n')
    let covClass = span.classList[0] === 'cov0' ? 'cov-uncovered' : 'cov-covered'

    for (let i = 0; i < lines.length; i++) {
      let trimmed = lines[i].trim()
      let [start, end] = lines[i].split(trimmed)

      if (trimmed[0] === '{' || trimmed[0] === '}') {
        trimmed = trimmed.slice(1).trim()
        start = lines[i].replace(trimmed, '')
      }

      if (trimmed[trimmed.length - 1] === '{' || trimmed[trimmed.length - 1] === '}') {
        trimmed = trimmed.slice(0, -1).trim()
        end = lines[i].split(trimmed)[1]
      }

      if (trimmed === '') {
        lines[i] = `${start || ''}${trimmed}${end || ''}`
      } else {
        lines[i] = `${start || ''}<span class="cov ${covClass}">${trimmed}</span>${end || ''}`
      }
    }

    span.innerHTML = lines.join('\n')
  })
}

function addIncrementalButton() {
  let url = window.location.href
  let isInc = url.includes('-inc.html')

  let link = document.createElement('a')
  link.classList = 'incremental'
  link.classList.add('hljs-selector-id')

  if (isInc) {
    link.title = 'Toggle to absolute coverage'
    link.href = url.replace('-inc.html', '.html')
  } else {
    link.title = 'Toggle to incremental coverage'
    link.href = url.replace('.html', '-inc.html')
  }

  link.innerHTML = `
    <span style="color: var(--covered);">↑</span>
    <span style="color: var(--uncovered);">↓</span>
    <span style="margin-left: 2px;">${isInc ? 'inc' : 'abs'}</span>
  `

  document.querySelector('#topbar').appendChild(link)
}

function configureFileSelect() {
  let selected = document.location.hash.slice(1)
  let files = document.getElementById('files')

  files.addEventListener('change', (e) => {
    let el = document.getElementById(e.target.value)

    if (!el) {
      files.value = 'file0'
      files.dispatchEvent(new Event('change'))
      return
    }

    document.location.hash = e.target.value

    document.body.scrollTop = 0;

    document.querySelectorAll('.file').forEach((el) => (el.style.display = 'none'))
    el.style.display = 'block'
  })

  files.value = selected
  files.dispatchEvent(new Event('change'))
}

function addThemeButton() {
  let isDark = localStorage.getItem('dark') === 'true'

  let switchInput = document.createElement('input')
  switchInput.type = 'checkbox'
  switchInput.id = 'switch'
  switchInput.checked = isDark

  let switchLabel = document.createElement('label')
  switchLabel.htmlFor = 'switch'

  document.querySelector('#topbar').appendChild(switchInput)
  document.querySelector('#topbar').appendChild(switchLabel)

  if (isDark) {
    toggleDarkMode()
  }

  document.querySelector('#switch').addEventListener('click', toggleDarkMode)
}

function toggleDarkMode() {
  let lightStyle = document.querySelector('link[href*="github.min.css"]')
  let darkStyle = document.querySelector('link[href*="github-dark.min.css"]')

  document.documentElement.classList.toggle('dark')

  if (document.documentElement.classList.contains('dark')) {
    localStorage.setItem('dark', 'true')
    lightStyle.setAttribute('disabled', 'disabled')
    darkStyle.removeAttribute('disabled')
  } else {
    localStorage.setItem('dark', 'false')
    lightStyle.removeAttribute('disabled')
    darkStyle.setAttribute('disabled', 'disabled')
  }
}

function configureCodeBlocks() {
  document.querySelectorAll('#content pre').forEach((pre) => {
    let gutter = document.createElement('div')
    gutter.classList.add('gutter')

    let editor = document.createElement('div')
    editor.classList.add('editor', 'language-go')
    editor.innerHTML = pre.innerHTML.replaceAll('    ', '  ')

    let code = document.createElement('div')
    code.appendChild(gutter)
    code.appendChild(editor)

    let coverage = code.cloneNode(true)
    coverage.classList = 'coverage'

    editor.innerHTML = editor.textContent
    code.classList = 'code'
    code.style.setProperty('position', 'absolute')
    code.style.setProperty('top', '0')
    code.style.setProperty('left', '0')

    pre.innerHTML = ''
    pre.appendChild(coverage)
    pre.appendChild(code)
  })
}

function configureSyntaxHighlight(cssSelector) {
  hljs.configure({ cssSelector, ignoreUnescapedHTML: true })
  hljs.highlightAll()
}

function addLineNumbers() {
  let pres = Array.from(document.querySelectorAll('#content pre'))

  pres.forEach((pre) => {
    let code = pre.querySelector('.coverage')
    let gutter = code.querySelector('.gutter')
    let editor = code.querySelector('.editor')
    let lines = editor.innerHTML.split('\n')
    let gutterHtml = ''

    // this function has two goals:
    // 1. add line numbers to the gutter
    // 2. assign a color to the line number based on the coverage of the line
    //
    // first, we add a .line-start span to each line in the editor.
    // this allows us to group the spans in the editor by line.
    // if a line has only one span, we assign the color of the span to the line number in the gutter.
    // if a line has more than one span and they have different background colors,
    // we can assume that the line has multiple statements with multiple coverage states
    // and we assign a yellow-ish color to the line number in the gutter.

    editor.innerHTML = lines.map((line) => `<span class="line-start"></span>${line}`).join('\n')

    let lineNumber = 1
    let spansInLine
    let spans = Array.from(editor.querySelectorAll('span'))

    for (let i = 0; i < spans.length; i++) {
      let currentSpan = spans[i]
      let nextSpan = spans[i + 1]

      if (currentSpan.classList.contains('line-start')) {
        spansInLine = []
      }

      if (nextSpan?.classList?.contains('cov')) {
        spansInLine.push(nextSpan)
        continue
      }

      if (!nextSpan?.classList.contains('line-start')) {
        continue
      }

      let classes = new Set(spansInLine.map((el) => el.classList[1]))
      let className = classes.size > 1 ? 'cov-mixed' : classes.values().next().value || ''

      gutterHtml += `<div class="ln ${className}">${lineNumber}</div>`

      lineNumber++
    }

    gutterHtml += `<div class="ln">${lineNumber}</div>`
    gutter.innerHTML = gutterHtml

    // add line numbers to the code gutter
    pre.querySelector('.code .gutter').innerHTML = gutterHtml
  })
}

function loadScript(src, state) {
  let script = document.createElement('script')
  script.src = src
  script.async = false
  script.onload = () => {
    console.info(`loaded: ${src}`)
    state.loaded++
  }
  document.head.appendChild(script)
}

function loadStyle(src, state) {
  let style = document.createElement('link')
  style.rel = 'stylesheet'
  style.href = src
  style.async = true
  style.onload = () => {
    console.info(`loaded: ${src}`)
    state.loaded++
  }
  document.head.appendChild(style)
}

function load(urls) {
  let state = {
    loaded: 0,
    isDone: () => state.loaded === urls.length,
  }

  for (let url of urls) {
    if (url.endsWith('.js')) {
      loadScript(url, state)
    } else {
      loadStyle(url, state)
    }
  }

  return state
}
