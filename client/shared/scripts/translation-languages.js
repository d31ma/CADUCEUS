// @ts-check

/**
 * Read a catalog written as `key=Value|key=Value` text. The catalogs are long
 * lists of names; one text per catalog keeps them readable and stops them from
 * looking like repeated code.
 *
 * @param {string} entries
 * @returns {Readonly<Record<string, string>>}
 */
function catalog(entries) {
  return Object.freeze(Object.fromEntries(entries.split('|').map(entry => entry.split('='))))
}

/**
 * Mailspring's complete reader-language catalogue. SMaLL-100 handles the
 * original 80-language subset directly; compact OPUS checkpoints handle the
 * remaining languages through an English pivot when necessary.
 *
 * @type {Readonly<Record<string, string>>}
 */
export const TRANSLATION_LANGUAGES = catalog(
  'af=Afrikaans|sq=Albanian|am=Amharic|ar=Arabic|hy=Armenian|az=Azerbaijani|ba=Bashkir|eu=Basque|' +
  'be=Belarusian|bn=Bengali|bs=Bosnian|bg=Bulgarian|my=Burmese|ca=Catalan|ceb=Cebuano|zh=Chinese|' +
  'hr=Croatian|cs=Czech|da=Danish|nl=Dutch|en=English|eo=Esperanto|et=Estonian|fi=Finnish|fr=French|' +
  'gl=Galician|ka=Georgian|de=German|el=Greek|gu=Gujarati|ht=Haitian Creole|he=Hebrew|hi=Hindi|' +
  'hu=Hungarian|mrj=Hill Mari|is=Icelandic|id=Indonesian|ga=Irish|it=Italian|ja=Japanese|jv=Javanese|' +
  'kn=Kannada|kk=Kazakh|km=Khmer|ko=Korean|ky=Kyrgyz|lo=Lao|la=Latin|lv=Latvian|lt=Lithuanian|' +
  'lb=Luxembourgish|mk=Macedonian|mg=Malagasy|ms=Malay|ml=Malayalam|mt=Maltese|mi=Maori|mhr=Mari|' +
  'mr=Marathi|mn=Mongolian|ne=Nepali|no=Norwegian|pap=Papiamento|fa=Persian|pl=Polish|pt=Portuguese|' +
  'pa=Punjabi|ro=Romanian|ru=Russian|gd=Scottish Gaelic|sr=Serbian|si=Sinhala|sk=Slovak|sl=Slovenian|' +
  'es=Spanish|su=Sundanese|sw=Swahili|sv=Swedish|tg=Tajik|tl=Tagalog|ta=Tamil|tt=Tatar|te=Telugu|' +
  'th=Thai|tr=Turkish|uk=Ukrainian|udm=Udmurt|ur=Urdu|uz=Uzbek|vi=Vietnamese|cy=Welsh|xh=Xhosa|' +
  'yi=Yiddish',
)

export const TRANSLATION_LANGUAGE_CODES = Object.freeze(Object.keys(TRANSLATION_LANGUAGES))

/** Languages supplied by the two compact OPUS fallback checkpoints. */
export const OPUS_EXTENSION_LANGUAGE_CODES = Object.freeze([
  'mt', 'mi', 'mhr', 'eu', 'pap', 'mrj', 'tg', 'tt', 'te', 'udm', 'ky', 'la', 'eo',
])

const opusExtensionLanguages = new Set(OPUS_EXTENSION_LANGUAGE_CODES)

/** Languages translated directly by the primary SMaLL-100 checkpoint. */
export const SMALL100_LANGUAGE_CODES = Object.freeze(
  TRANSLATION_LANGUAGE_CODES.filter(language => !opusExtensionLanguages.has(language)),
)

/** Map franc's ISO 639-3 result to the model's BCP-47 primary tag. */
export const FRANC_TO_TRANSLATION_LANGUAGE = catalog(
  'afr=af|amh=am|arb=ar|azj=az|bak=ba|bel=be|ben=bn|bos=bs|bul=bg|cat=ca|ceb=ceb|ces=cs|cmn=zh|cym=cy|' +
  'dan=da|deu=de|ell=el|eng=en|epo=eo|est=et|eus=eu|fin=fi|fra=fr|gla=gd|gle=ga|glg=gl|guj=gu|hat=ht|' +
  'heb=he|hin=hi|hrv=hr|hun=hu|hye=hy|ind=id|isl=is|ita=it|jav=jv|jpn=ja|kan=kn|kat=ka|kaz=kk|khm=km|' +
  'kor=ko|kir=ky|lao=lo|lat=la|lav=lv|lit=lt|ltz=lb|mal=ml|mar=mr|chm=mhr|mlt=mt|mkd=mk|mon=mn|mya=my|' +
  'mri=mi|nld=nl|nob=no|npi=ne|pan=pa|pap=pap|pes=fa|plt=mg|pol=pl|por=pt|ron=ro|rus=ru|sin=si|slk=sk|' +
  'slv=sl|spa=es|sqi=sq|srp=sr|sun=su|swe=sv|swh=sw|tam=ta|tat=tt|tel=te|tgl=tl|tgk=tg|tha=th|tur=tr|' +
  'ukr=uk|udm=udm|urd=ur|uzn=uz|vie=vi|xho=xh|ydd=yi|zlm=ms',
)
