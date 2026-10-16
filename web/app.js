const elements = {
  input: document.querySelector('#file-input'),
  dropZone: document.querySelector('#drop-zone'),
  preview: document.querySelector('#preview'),
  fileName: document.querySelector('#file-name'),
  fileSize: document.querySelector('#file-size'),
  language: document.querySelector('#language'),
  runButton: document.querySelector('#run-button'),
  result: document.querySelector('#result'),
  count: document.querySelector('#character-count'),
  status: document.querySelector('#status'),
  copyButton: document.querySelector('#copy-button')
};

const MAX_FILE_SIZE = 10 * 1024 * 1024;
const ACCEPTED_TYPES = new Set(['image/png', 'image/jpeg']);
let selectedFile = null;
let previewUrl = '';

function formatSize(bytes) {
  if (bytes < 1024 * 1024) return `${Math.max(1, Math.round(bytes / 1024))} Ko`;
  return `${(bytes / (1024 * 1024)).toFixed(1).replace('.', ',')} Mo`;
}

function setStatus(message, state = '') {
  elements.status.dataset.state = state;
  elements.status.lastElementChild.textContent = message;
}

function updateCount() {
  const count = elements.result.value.length;
  elements.count.textContent = `${count.toLocaleString('fr-FR')} caractère${count > 1 ? 's' : ''}`;
  elements.copyButton.disabled = count === 0;
}

function selectFile(file) {
  if (!file) return;
  if (!ACCEPTED_TYPES.has(file.type)) {
    setStatus('Choisissez une image PNG ou JPEG.', 'error');
    return;
  }
  if (file.size > MAX_FILE_SIZE) {
    setStatus('Cette image dépasse la limite de 10 Mo.', 'error');
    return;
  }

  selectedFile = file;
  if (previewUrl) URL.revokeObjectURL(previewUrl);
  previewUrl = URL.createObjectURL(file);
  elements.preview.src = previewUrl;
  elements.preview.alt = `Aperçu de ${file.name || 'l’image collée'}`;
  elements.fileName.textContent = file.name || 'Image collée';
  elements.fileSize.textContent = formatSize(file.size);
  elements.dropZone.classList.add('has-image');
  elements.runButton.disabled = false;
  setStatus('Image prête pour l’extraction.', 'ready');
}

elements.dropZone.addEventListener('click', () => elements.input.click());
elements.dropZone.addEventListener('keydown', event => {
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault();
    elements.input.click();
  }
});
elements.input.addEventListener('change', () => selectFile(elements.input.files[0]));

['dragenter', 'dragover'].forEach(type => elements.dropZone.addEventListener(type, event => {
  event.preventDefault();
  elements.dropZone.classList.add('is-dragging');
}));
['dragleave', 'drop'].forEach(type => elements.dropZone.addEventListener(type, event => {
  event.preventDefault();
  elements.dropZone.classList.remove('is-dragging');
}));
elements.dropZone.addEventListener('drop', event => selectFile(event.dataTransfer.files[0]));

document.addEventListener('paste', event => {
  const item = [...(event.clipboardData?.items || [])].find(entry => entry.type.startsWith('image/'));
  if (item) {
    event.preventDefault();
    selectFile(item.getAsFile());
  }
});

elements.runButton.addEventListener('click', async () => {
  if (!selectedFile) return;
  elements.runButton.disabled = true;
  elements.dropZone.classList.add('is-processing');
  setStatus('Reconnaissance en cours…', 'loading');

  const payload = new FormData();
  payload.append('image', selectedFile);
  payload.append('lang', elements.language.value);

  try {
    const response = await fetch('/api/ocr', { method: 'POST', body: payload });
    const data = await response.json().catch(() => ({}));
    if (!response.ok) throw new Error(data.error || 'Le serveur n’a pas pu traiter cette image.');
    elements.result.value = data.text || '';
    updateCount();
    if (data.text) setStatus(`Texte extrait en ${(data.duration_ms / 1000).toFixed(2).replace('.', ',')} s.`, 'success');
    else setStatus('Aucun texte lisible n’a été détecté.', 'ready');
  } catch (error) {
    setStatus(error.message || 'La reconnaissance a échoué. Réessayez.', 'error');
  } finally {
    elements.runButton.disabled = false;
    elements.dropZone.classList.remove('is-processing');
  }
});

elements.result.addEventListener('input', updateCount);
elements.copyButton.addEventListener('click', async () => {
  if (!elements.result.value) return;
  try {
    await navigator.clipboard.writeText(elements.result.value);
    setStatus('Texte copié dans le presse-papiers.', 'success');
  } catch {
    elements.result.focus();
    elements.result.select();
    setStatus('Texte sélectionné. Utilisez Ctrl+C pour le copier.', 'ready');
  }
});

window.addEventListener('beforeunload', () => {
  if (previewUrl) URL.revokeObjectURL(previewUrl);
});
