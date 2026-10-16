const VERSION = 'f5f6dae276891216'
const CACHE = 'tachyon-static-' + VERSION
const PREFIX = 'tachyon-static-'
// Policies the project declared by path in client/manifest.json, compiled to anchored
// expressions at build time so the worker needs no glob parser.
const RULES = ([]).map((rule) => ({ ...rule, match: new RegExp(rule.pattern) }))
// Only byte-verified build outputs may bootstrap anonymously from a browser's
// ordinary document/module requests. An extension is not a privacy boundary.
const ASSETS = {"/":{"bytes":146869,"sha256":"edbcdb9d02c8bc5ba8967717f42728baec11c7d2bb45ae0f0ee7f52fe5dd2bb3"},"/.tachyon/behavior-runtime.js":{"bytes":53070,"sha256":"2897d1831074480e9429f3e9fb8131a23707c5a43a7674534c3178ea5587e9bd"},"/.tachyon/components.css":{"bytes":97827,"sha256":"7d92cd9c36f03d4888e6013c633455a3426b85ce5eb3c261ef54c6bc43123e49"},"/.tachyon/navigation.css":{"bytes":93,"sha256":"f8951b2fd3b58ce1732efc1e7de70b0b25a0024684642aced5b2556849284cb7"},"/.tachyon/register-sw.js":{"bytes":766,"sha256":"263657820da833d169b70401ff7aa9c2446bfc14f41ca5dbb805c6f4188c9ed2"},"/.tachyon/services/ActivityService.js":{"bytes":527,"sha256":"0962c10a780425f4babe866f47baa049876d7991976db46c41b186fd50e8247d"},"/.tachyon/services/BrowserPageService.js":{"bytes":25644,"sha256":"5c56cf26108424fca6601e26f57cf29a42e9eef81c1e26183ecba3295ea99689"},"/.tachyon/services/BrowserStorageService.js":{"bytes":18785,"sha256":"82c8f454b971f63cdd4f0589616f1df154ec9dafe755ca8717d007f765956868"},"/.tachyon/services/CategoryPickerService.js":{"bytes":551,"sha256":"05e270309f76f42bf0da53b1df43201ee4ac69ed79a2f81d2408d2277849c3a4"},"/.tachyon/services/ConfirmationService.js":{"bytes":17395,"sha256":"44e83c085a8d8612e336c7f8a03be6228100c219f2283e3d4b61ced81848c557"},"/.tachyon/services/DeviceService.js":{"bytes":21447,"sha256":"8922b4e5788d60396f02ae0ed777ecb5c0a19d29ba15f8201958c910baa4ec84"},"/.tachyon/services/ExternalMailboxService.js":{"bytes":555,"sha256":"27855ba54db04fce709137108009c3b1aa1805a3a39678f243abdeca0120d89c"},"/.tachyon/services/InboxService.js":{"bytes":515,"sha256":"cee0a07f2cb7d83b72366580615be544291005e83daaa67484307b2feff4d139"},"/.tachyon/services/KeyboardService.js":{"bytes":19230,"sha256":"b84764b129b31ad19f8c879eca784f01fc2e6f309f8c52592e6e157b760f1b29"},"/.tachyon/services/LoginService.js":{"bytes":515,"sha256":"b8090fa1d4e561e21bbd5fad66ef62584336a0daa7583e6a0cf980cfb514902a"},"/.tachyon/services/MailLayoutService.js":{"bytes":535,"sha256":"7dd5e6763b4040de95a4436fa85b0b6f07ce424be9eeb04cb8b378ec53a2bab1"},"/.tachyon/services/MailboxArchiveService.js":{"bytes":23301,"sha256":"58421f482d0a1d3f369b23cdc5878a13feab6d520a34b7530e058671f8e5ed9d"},"/.tachyon/services/MailboxBrowserService.js":{"bytes":23746,"sha256":"5bfd336b1fc38139ddf4c3acff99beeef97c37faad1425e79ee766cd1f0cd677"},"/.tachyon/services/MailboxService.js":{"bytes":523,"sha256":"44c6665cb4e3bc64cd1e04b40fc52cc1a5a9c80a4bf2672b3786cc6787677f2a"},"/.tachyon/services/MailboxTransferService.js":{"bytes":555,"sha256":"d8054e954ee2c33737f26e8943134d13fa67d414eb243969126217b3e8701317"},"/.tachyon/services/NavigationService.js":{"bytes":535,"sha256":"2d0e14927e5487ee7496abab949d04bb24a2312fd293e86278d89fce13ba8cf2"},"/.tachyon/services/NotificationCenterService.js":{"bytes":567,"sha256":"1bc94fac331186543459f6fdd73a97ba7980f95fcd688239dfb04fa3c8aa8c80"},"/.tachyon/services/PasskeyService.js":{"bytes":21586,"sha256":"0c89b8474fe03411eeb9d14b87c1baca567b2793ad10ce897e535086eeb0c409"},"/.tachyon/services/PreferenceService.js":{"bytes":535,"sha256":"17aa2d0d4b5aaaa93e94189f81e20bae380cae1f5a02c7d7d35e8d81f8620a80"},"/.tachyon/services/RemindersService.js":{"bytes":531,"sha256":"144088267dbe6f57daecb9def1405ec83a6cdde86cdb1b6068b3fb9c9266ca7b"},"/.tachyon/services/ScheduledService.js":{"bytes":531,"sha256":"bc9602339fb1bba52875fc683e31ac327c0156070d03aa29ed30245982e292a1"},"/.tachyon/services/SessionService.js":{"bytes":523,"sha256":"77f790d97e7020ee0df3407af314cb11d04ddb4015211e538f3c5338365da548"},"/.tachyon/services/ShareService.js":{"bytes":515,"sha256":"b6ebe59e771ee33a5c2a938800cb2d35ed17652aecb3afcc2788386e3769045f"},"/.tachyon/services/ShellService.js":{"bytes":515,"sha256":"93f98da9e304a8a470a3e678830638487ffeb00bd9843b33c53eafdb1aa87501"},"/.tachyon/services/TeamService.js":{"bytes":511,"sha256":"44f1cf7de32e521e4c49648111e1a792b1601343b58f39664a90cba3f95f35eb"},"/.tachyon/services/ThreadSearchService.js":{"bytes":543,"sha256":"e0f35031c39f9b2df352c02c28303cc96d5c5ace58d78401a7fc44f08434f019"},"/.tachyon/services/go/services.wasm":{"bytes":3345205,"sha256":"e419cf11ed27fa1176704c4cdb8c912b52ae95ebcd46fb540c13426a6d73ec74"},"/.tachyon/services/go/wasm_exec.js":{"bytes":13542,"sha256":"d83ecea6495604c2c8778cdcec16e488991ded35328cd638e7cb8b7c33264f4b"},"/.tachyon/services/tachyon-go.js":{"bytes":9147,"sha256":"e761d8c63358c3436e5f221b8032747e5b1d00b03e26402b3bd1fd8f5590eb84"},"/.tachyon/source-maps/activity.map.json":{"bytes":138,"sha256":"13a4426181cd69de9fc70f5d27a08011209951ff0dfb527e43751f7799c3eb46"},"/.tachyon/source-maps/folder___name.map.json":{"bytes":146,"sha256":"57521bd18ca51ed974182196ee412d03a281e4e084aaaa60ca173a070106f538"},"/.tachyon/source-maps/inbox.map.json":{"bytes":132,"sha256":"5dfbe5767e066e169d7462affa7cc883513d6ab00b6bc5efdc2314fc113218c0"},"/.tachyon/source-maps/reminders.map.json":{"bytes":140,"sha256":"77805721075b4b4b83ef0021fb503dcfd22279f1e6df760fbf85abc6f94cc613"},"/.tachyon/source-maps/root.map.json":{"bytes":120,"sha256":"e57307abf588c2c4d8a19149db8f517afb6d776cfeb8ab2c3ec130e729d36d05"},"/.tachyon/source-maps/scheduled.map.json":{"bytes":140,"sha256":"e684a69bc982378f4572167a30d50b0f2a532fed1830c4e528475b23eab99cbc"},"/.tachyon/source-maps/share___token.map.json":{"bytes":146,"sha256":"55ede88d114680b5c415a36dafb937f90d4e492783eebfee1c56c06bfcd9e0bb"},"/.tachyon/source-maps/team.map.json":{"bytes":130,"sha256":"9668de71bd4bec800b4771239ac50e070e868ef330df2d68363224674fdfda9b"},"/.tachyon/tac-client.js":{"bytes":123988,"sha256":"9a7b8b3a2f9bf7719320791ff052a6f098e31b3ea371b568623094393f885002"},"/.tachyon/view-ir/activity.json":{"bytes":907,"sha256":"9a269c5bf3cb7dc8502703d90d59128ba60e92be48115e33abc51a917294b062"},"/.tachyon/view-ir/folder___name.json":{"bytes":910,"sha256":"1a2a90529d7c8bcea480b018122ebd3f41d8472e6a789e9c7e6ffb6728e539f1"},"/.tachyon/view-ir/inbox.json":{"bytes":903,"sha256":"71eb0df0c40a20229100c14ee537ff5d058103ba38aac0f67ebd552525177f99"},"/.tachyon/view-ir/reminders.json":{"bytes":909,"sha256":"d8f917725363abfd96ba24c51795816cf52e6fb12e05d768073c12755ef54f98"},"/.tachyon/view-ir/root.json":{"bytes":520,"sha256":"f22997427cbf61876d7d219fc3d3a0b0215aac054a3a99d43ce9b9c7bd334bcb"},"/.tachyon/view-ir/scheduled.json":{"bytes":909,"sha256":"a568cc9c3b56d61ef5725acc687ae4c45967ed66470c5f78b4438f638430a469"},"/.tachyon/view-ir/share___token.json":{"bytes":587,"sha256":"26dcfec94acc070e1ca151688ab4e61c8ea4bf7ce208bdfc99f79894b20b8090"},"/.tachyon/view-ir/team.json":{"bytes":956,"sha256":"ed0c2792e9a7f3baa804c52aa8c2cd0b15d1ae6cfa9220554dfc965795ec383d"},"/activity":{"bytes":174429,"sha256":"0d88757c916d321ff6b5428a8460693600328330b071bcfd686d9b0d40bbc6ae"},"/activity/":{"bytes":174429,"sha256":"0d88757c916d321ff6b5428a8460693600328330b071bcfd686d9b0d40bbc6ae"},"/activity/index.html":{"bytes":174429,"sha256":"0d88757c916d321ff6b5428a8460693600328330b071bcfd686d9b0d40bbc6ae"},"/api-docs":{"bytes":606,"sha256":"1650500d80dba5746afe6308fcb97cc7e924e5976ce2eb62cd3c60edee6f503d"},"/api-docs/":{"bytes":606,"sha256":"1650500d80dba5746afe6308fcb97cc7e924e5976ce2eb62cd3c60edee6f503d"},"/api-docs/index.html":{"bytes":606,"sha256":"1650500d80dba5746afe6308fcb97cc7e924e5976ce2eb62cd3c60edee6f503d"},"/api-docs/viewer.css":{"bytes":1575,"sha256":"148dff0f650abfc857e7ce05685dfa0bc416525203d1842b8e54172438bcf041"},"/api-docs/viewer.js":{"bytes":6078,"sha256":"0b4611ccf487fe0e14bf3eb3104d52aa68c383d13e5eb4668051b2f7a234bc0c"},"/api.json":{"bytes":55787,"sha256":"d316d314dd32333bc0256b4106a1f43926cf8c45565f1addb5e9060d6678bdc8"},"/folder/_name":{"bytes":240905,"sha256":"156acd18b28422b97579ede7a343cea3950e2872eb23b51e50b416ec145d6449"},"/folder/_name/":{"bytes":240905,"sha256":"156acd18b28422b97579ede7a343cea3950e2872eb23b51e50b416ec145d6449"},"/folder/_name/index.html":{"bytes":240905,"sha256":"156acd18b28422b97579ede7a343cea3950e2872eb23b51e50b416ec145d6449"},"/inbox":{"bytes":240898,"sha256":"b265996f993fcd0dc177cd5d7970099315098884ed72beedd33d471ce8e5d573"},"/inbox/":{"bytes":240898,"sha256":"b265996f993fcd0dc177cd5d7970099315098884ed72beedd33d471ce8e5d573"},"/inbox/index.html":{"bytes":240898,"sha256":"b265996f993fcd0dc177cd5d7970099315098884ed72beedd33d471ce8e5d573"},"/index.html":{"bytes":146869,"sha256":"edbcdb9d02c8bc5ba8967717f42728baec11c7d2bb45ae0f0ee7f52fe5dd2bb3"},"/manifest.webmanifest":{"bytes":3204,"sha256":"46b86387ad776a30c0f57ed5a58703da7d392cae540a15d0f8d906764256080e"},"/reminders":{"bytes":155699,"sha256":"a27e86b92ada8a74b558e9e9578c2365f30075ec43d4c5c23be5b7732a7e68dd"},"/reminders/":{"bytes":155699,"sha256":"a27e86b92ada8a74b558e9e9578c2365f30075ec43d4c5c23be5b7732a7e68dd"},"/reminders/index.html":{"bytes":155699,"sha256":"a27e86b92ada8a74b558e9e9578c2365f30075ec43d4c5c23be5b7732a7e68dd"},"/route-manifest.json":{"bytes":63597,"sha256":"ceeaacddc99f7b5a9694c7694ddfab803a3592c0e35cdf50b7294be73228fb80"},"/scheduled":{"bytes":158095,"sha256":"10c9b829fcf51df15aa5da6a7a4d167fa1252d3d5935e40f16d4851783fdc707"},"/scheduled/":{"bytes":158095,"sha256":"10c9b829fcf51df15aa5da6a7a4d167fa1252d3d5935e40f16d4851783fdc707"},"/scheduled/index.html":{"bytes":158095,"sha256":"10c9b829fcf51df15aa5da6a7a4d167fa1252d3d5935e40f16d4851783fdc707"},"/share/_token":{"bytes":15842,"sha256":"88acc3ea0f27a4878a955f4ed2ab80baea6a8a8fad6e4254efb1979629296605"},"/share/_token/":{"bytes":15842,"sha256":"88acc3ea0f27a4878a955f4ed2ab80baea6a8a8fad6e4254efb1979629296605"},"/share/_token/index.html":{"bytes":15842,"sha256":"88acc3ea0f27a4878a955f4ed2ab80baea6a8a8fad6e4254efb1979629296605"},"/shared/assets/config.js":{"bytes":419,"sha256":"ef1353495064f1cc77eade5bba8eccd0efdd167908c9d7ece9645edb2d35b5ae"},"/shared/assets/duvay/duvay-wc.min.js":{"bytes":469681,"sha256":"67f441e7d55652f6f17a8205e8d8249b980e85533313f79a9934898dfc51a763"},"/shared/assets/duvay/duvay.min.css":{"bytes":464991,"sha256":"67f77f4c8beaa6c3372af0cc54ff127cdeaa4ad9f1314a2aea48a58a1ade9027"},"/shared/assets/favicon.svg":{"bytes":454,"sha256":"21a9542a9c68afda53b7c936bbb44cea05e95c477bccfba320af07636c97ac96"},"/shared/assets/icon-192.png":{"bytes":1021,"sha256":"0169bec6cc341c6e65283b82e69c58ebe259dd001c149342a0a029f6e453ea3b"},"/shared/assets/icon-512.png":{"bytes":3845,"sha256":"08b59c6d2bb14eda423d7f3acc116957d8777ab0ef859fb71b6a99311663afed"},"/shared/assets/interface-locales.json":{"bytes":1019596,"sha256":"e6ed8a6831c112849c522d1caaba8458467aa9c53af368264faf4e7e3d2246dc"},"/shared/assets/maskable-icon-512.png":{"bytes":3497,"sha256":"85fb76f71e4c96a14065070a40aa2c93c38335690af5c81b2f9494634ea11110"},"/shared/assets/print.css":{"bytes":2510,"sha256":"d00c37eb9412af6274a57beb116a63d61f3352cc8744f1cdf0c517aa5da22f6d"},"/shared/assets/styles.css":{"bytes":6285,"sha256":"0928630f76aa5f3aeb23025d8ed627395bd959e93a6713d52bd72a255e769842"},"/shared/assets/themes.css":{"bytes":7857,"sha256":"c19e40b2976f8c47aa31bf94b426bbae8ec46c34a29fce26470be75e1addd43b"},"/shared/data/keymap.json":{"bytes":6623,"sha256":"dedd08d7146304b58e201891d115c7575c0b3bada5e4a8232e28461613a14969"},"/shared/locales/ar.js":{"bytes":2319,"sha256":"474a7f20ac0218d5d0b90ba1ed0c597c4e70e7b511e6b008faa2354c26d35618"},"/shared/locales/en.js":{"bytes":1684,"sha256":"9fb0348544b99015e2304889df1361a61bbc479bf8cdfa26dc1808dc32522069"},"/shared/locales/fr.js":{"bytes":1921,"sha256":"1618969b6dad5fe415ff2dbb330eed4b574da5ef90c5a76afaff487675f1d17b"},"/shared/scripts/attachment-content.js":{"bytes":2749,"sha256":"8ee8e7d1ff6bf6eb73b3ae658abd00e39008e169a83c2c2bc048e130f8978dfc"},"/shared/scripts/calendar-date.js":{"bytes":5209,"sha256":"a1424d61b8c4dbc4456b9a174767e7456bb830a55234cae6dcb5aa64d7807cc7"},"/shared/scripts/category-picker.js":{"bytes":2548,"sha256":"bd2b39491f8b6403797b7dcae515f00edc1ee2358887359e20c091ab768b083c"},"/shared/scripts/chunked-asset-fetch.js":{"bytes":6300,"sha256":"e5c8ddbbd4b9c2d81cf42b5a797592ce07e328b4f20b5aede8015c9541936ab9"},"/shared/scripts/composer-signatures.js":{"bytes":3682,"sha256":"fc8de64ee0ec5dbdac48ee0d555d255d5c179ed5e6c590f021b5091ad5a30753"},"/shared/scripts/composer-templates.js":{"bytes":5837,"sha256":"46ef9ee6778430f726f600819c74d1eb3ec05b2e72a1d75e164a15504da30ce6"},"/shared/scripts/dialog-focus.js":{"bytes":4032,"sha256":"17bb7d76f9260d78ae9e6cb968ce80c36f44bb46896c55a5859cdf310b4ca7da"},"/shared/scripts/eml-archive.js":{"bytes":7408,"sha256":"ff53dd200411bdcff0120c3bf98120f10417988774fde94752ceb5cdc5b1064d"},"/shared/scripts/external-accounts-ui.js":{"bytes":6664,"sha256":"21115f7136e82fc37f41f238fdf6bf27a67d52826f08e0aa01035085a962cabf"},"/shared/scripts/external-folder-tree.js":{"bytes":5886,"sha256":"e8d92585115f709ebb6f481f0582ea8f100347fd3bc62b27f59c2faf98086543"},"/shared/scripts/fetch-cache.js":{"bytes":5888,"sha256":"6a107e6289991f7e9f86d94159c3194f4342088bef60cb11105e7883a2a2b8ec"},"/shared/scripts/github-notification.js":{"bytes":513,"sha256":"b30b5b2a2fef747d5e6120054b3ac316a90c5b4ff37f0a4d70c1c97c5f7dde81"},"/shared/scripts/grammar-check.js":{"bytes":4073,"sha256":"aecf0e352698e0ef14b205fa0b54e2bc37fcd2abbda722bb655b225a5a8ef454"},"/shared/scripts/interface-localizer.js":{"bytes":4501,"sha256":"145ef2e523db76ae0147dc57f01a0fa290ac2508dda3cbf9469589c908a1a539"},"/shared/scripts/keyboard.js":{"bytes":12041,"sha256":"d2fe1fff483907b22821c66540bccdc8f3e53a20a2bba0cc0150eb6f38061b8f"},"/shared/scripts/localization.js":{"bytes":10479,"sha256":"8193b142046e6a9ae19f07063e3bbb8f271fdf972b6c41177ba92784363f2e17"},"/shared/scripts/mail-preferences.js":{"bytes":5163,"sha256":"d584e98997cca7e1a58c8c910faa9f77304c7380b29d4694c97fb8adf422e3e7"},"/shared/scripts/mailbox-archive.js":{"bytes":10932,"sha256":"1d1939b327adca03e22ee55bfe1fcaf4b54a9e17bf007081ce0428b05cce9d12"},"/shared/scripts/mailbox-browser.js":{"bytes":12014,"sha256":"7bc8531851d9cdac8b06b8017ad555a1e03baa5bcd69e752d0de61d6d57abe05"},"/shared/scripts/mbox.js":{"bytes":12115,"sha256":"40fc574b5dd8002ff81a0ac89d2dd71fe93837ffb83a207b0c860895e9ea208e"},"/shared/scripts/message-conversation.js":{"bytes":4411,"sha256":"29db79f772c9d6b5213fa264d41f57a98e84923906e4d0cb97cc52b4fd6d37ca"},"/shared/scripts/notification-sounds.js":{"bytes":5405,"sha256":"05d9ee15b04b19c63544e93dc2e8724cf9552e3fa57281953ec32f1e4ed887aa"},"/shared/scripts/offline-mailbox-index.js":{"bytes":12721,"sha256":"2fac7c97435c8b1c5bf3409c8042b55b7a4e6de610856c40777937195edbd115"},"/shared/scripts/offline-write-vault.js":{"bytes":14863,"sha256":"53ff818500bbde45881e4f1af3cb0ae923dd3ad35c481bfb40b90ce048161e50"},"/shared/scripts/page-setup.js":{"bytes":2368,"sha256":"58bf0ee4ab2f5a4344c4bcbda449da9a9b7b61a34e4260da359a34841fa5fa73"},"/shared/scripts/print-conversation.js":{"bytes":7904,"sha256":"d6075f02428b4575b0075afc484927df5a4095ead803f1abe9eb7250ef0b07f3"},"/shared/scripts/pwa-launch.js":{"bytes":2637,"sha256":"386f5f34e1b4e7dc74126253f4f79607487d94b19cbe0112a2938ecc6ebdc138"},"/shared/scripts/shortcuts.js":{"bytes":8844,"sha256":"54d9bec31b73f369d1ccf074f431421cd62fe6dc064f221d3724e9957a2ac264"},"/shared/scripts/snooze-presets.js":{"bytes":2057,"sha256":"8dde03b2d1da5081e46fbe75557e3ae7126132d5505cb32f54a7ad4f345fe92a"},"/shared/scripts/theme-preferences.js":{"bytes":5644,"sha256":"b2e412c7bdebe5967d9af083ff264351279281fdf0054c6994d633216299f941"},"/shared/scripts/thread-search.js":{"bytes":4181,"sha256":"33c578fae663d4ff7e0d49e7b3c5bea58340d23255aa7599c05161961f936c19"},"/shared/scripts/translation-cache.js":{"bytes":5228,"sha256":"f9f89039093df8018c36102f39f639af308054d63322f29eefe12b09dce3b2ae"},"/shared/scripts/translation-detection-vendor.js":{"bytes":247,"sha256":"e0878c2e4594c2db026eaa3f056193500f02691b734b7c83878513cbed0562b3"},"/shared/scripts/translation-engine-vendor.js":{"bytes":258,"sha256":"639db7647512a84b816e8dcab8bd8e2e35c293a4152c7c083a1f5d1a9e540918"},"/shared/scripts/translation-preferences.js":{"bytes":2833,"sha256":"96e382e27d4586b909a5b449412b9bb0b106dcdbda7a8234c0113d890cf4c30d"},"/shared/scripts/unread-badge.js":{"bytes":9066,"sha256":"d01ef9a183db57264dd404b3a27467977ecb68dd38376f30284f6aaa145ef5c7"},"/shared/scripts/unsubscribe.js":{"bytes":2043,"sha256":"ad1347e537ded5461ab16653e473e56c773909b36fe6a5fe158d6e3a4bbf0767"},"/shared/service-worker/unread.js":{"bytes":5196,"sha256":"d041f6a610a65dfdac53c8f3790dcc05fa3116f2e8d4c86a305d9c317c5310fa"},"/shared/styles/.gitkeep":{"bytes":1,"sha256":"01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b"},"/spa-renderer.js":{"bytes":101,"sha256":"22f707775555f80de117ec910b5bcd616c36428c8e59ca5f89ee45678165e1c6"},"/team":{"bytes":152968,"sha256":"891a68de2dc6898e7c6c8a1fdf8f877826af843b166f9effdb803b467f26e8f5"},"/team/":{"bytes":152968,"sha256":"891a68de2dc6898e7c6c8a1fdf8f877826af843b166f9effdb803b467f26e8f5"},"/team/index.html":{"bytes":152968,"sha256":"891a68de2dc6898e7c6c8a1fdf8f877826af843b166f9effdb803b467f26e8f5"}}
const MAX_ENTRIES = 256
const MAX_BYTES = 32 * 1024 * 1024
const MAX_AGE = 24 * 60 * 60 * 1000
let writes = Promise.resolve()
let pendingWrites = 0

const bounded = async (work) => {
  let timer
  try { return await Promise.race([work, new Promise((_, reject) => { timer = setTimeout(() => reject(new Error('cache unavailable')), 2000) })]) }
  finally { clearTimeout(timer) }
}
const publicResponse = (response) => response && response.ok && !response.redirected
  && response.type !== 'opaque' && response.type !== 'opaqueredirect'
  && !/(?:^|,)\s*(?:private|no-store)\b/i.test(response.headers.get('cache-control') || '')
  && !response.headers.has('vary')

async function bodyBytes(response, limit) {
  const reader = response.body?.getReader()
  if (!reader) return new Uint8Array()
  const chunks = []
  let length = 0
  const deadline = Date.now() + 2000
  try {
    while (true) {
      if (Date.now() >= deadline) throw new Error('cache body deadline')
      const { value, done } = await bounded(reader.read())
      if (done) break
      length += value.byteLength
      if (length > limit) throw new Error('cache body limit')
      chunks.push(value)
    }
  } finally { void reader.cancel().catch(() => {}) }
  const bytes = new Uint8Array(length)
  let offset = 0
  for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength }
  return bytes
}
async function matchesAsset(bytes, asset) {
  if (bytes.byteLength !== asset.bytes) return false
  const digest = await bounded(crypto.subtle.digest('SHA-256', bytes))
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('') === asset.sha256
}
async function evict(request) {
  try { const cache = await bounded(caches.open(CACHE)); await bounded(cache.delete(request)) } catch {}
}

self.addEventListener('install', (event) => { event.waitUntil(self.skipWaiting()) })

self.addEventListener('activate', (event) => {
  event.waitUntil((async () => {
    // Assets a previous build referenced expire with their cache.
    for (const name of await caches.keys())
      if (name.startsWith(PREFIX) && name !== CACHE) await caches.delete(name)
    await self.clients.claim()
  })())
})

self.addEventListener('fetch', (event) => {
  const request = event.request
  const url = new URL(request.url)
  // Cross-origin requests and the live-reload channel are left alone.
  if (url.origin !== self.location.origin) return
  if (url.pathname.startsWith('/.tachyon/live-reload') || url.pathname === '/.tachyon/hot') return

  if (request.method !== 'GET' && request.method !== 'HEAD') {
    event.respondWith(writeThrough(request, url))
    return
  }

  if (request.method === 'HEAD') return

  const rule = RULES.find((candidate) => candidate.match.test(url.pathname))
  const asset = !url.search && Object.hasOwn(ASSETS, url.pathname) ? ASSETS[url.pathname] : null
  // This decision precedes lookup as CacheStorage does not key by credentials.
  if (request.cache === 'no-store' || request.headers.has('authorization')
      || request.headers.has('range') || rule?.policy === 'no-store'
      || (!asset && (!rule || request.credentials !== 'omit'))) {
    event.respondWith((async () => { await evict(request); return fetch(request) })())
    return
  }
  const revalidate = !asset && request.cache === 'no-cache'
  const cacheFirst = !revalidate && (rule ? rule.policy === 'cache-first' : request.mode !== 'navigate')
  event.respondWith(cacheFirst ? fromCache(request, asset) : fromNetwork(request, asset))
})

async function writeThrough(request, url) {
  const response = await fetch(request)
  if (!response.ok) return response
  try {
    const cache = await bounded(caches.open(CACHE))
    for (const entry of await bounded(cache.keys())) {
      const path = new URL(entry.url).pathname
      if (path === url.pathname || url.pathname.startsWith(path + '/')) await bounded(cache.delete(entry))
    }
  } catch {}
  return response
}

async function cacheRead(request, asset) {
  try {
    const cache = await bounded(caches.open(CACHE))
    const response = await bounded(cache.match(request))
    if (!response) return null
    const created = Number(response.headers.get('x-tachyon-cached-at'))
    if (!publicResponse(response) || (!asset && (!created || Date.now() - created > MAX_AGE))) {
      await bounded(cache.delete(request)); return null
    }
    if (asset && !await matchesAsset(await bodyBytes(response.clone(), asset.bodyLimit ?? 4 * 1024 * 1024), asset)) {
      await bounded(cache.delete(request)); return null
    }
    return response
  } catch { return null }
}

async function cacheWrite(request, response, asset) {
  if (!publicResponse(response)) { await evict(request); return }
  if (pendingWrites >= 16) return
  pendingWrites += 1
  try {
    const bytes = await bodyBytes(response.clone(), asset ? (asset.bodyLimit ?? 4 * 1024 * 1024) : 256 * 1024)
    if (asset && !await matchesAsset(bytes, asset)) { await evict(request); return }
    const headers = new Headers(response.headers)
    headers.delete('content-encoding')
    headers.set('content-length', String(bytes.byteLength))
    headers.set('x-tachyon-cache-bytes', String(bytes.byteLength))
    headers.set('x-tachyon-cached-at', String(Date.now()))
    const stored = new Response(bytes, { status: response.status, statusText: response.statusText, headers })
    writes = writes.catch(() => {}).then(async () => {
      const cache = await bounded(caches.open(CACHE))
      const entries = await bounded(cache.keys())
      let size = bytes.byteLength
      for (const entry of entries) {
        const existing = await bounded(cache.match(entry))
        const known = ASSETS[new URL(entry.url).pathname]
        // Runtime-prewarmed build assets have a known size even without metadata.
        size += Number(existing?.headers.get('x-tachyon-cache-bytes')) || known?.bytes || MAX_BYTES
      }
      if (entries.length >= MAX_ENTRIES || size > MAX_BYTES)
        for (const entry of entries) await bounded(cache.delete(entry))
      await bounded(cache.put(request, stored))
    })
    await writes
  } catch { await evict(request) }
  finally { pendingWrites -= 1 }
}

async function fromCache(request, asset) {
  return (await cacheRead(request, asset)) || fromNetwork(request, asset)
}

async function fromNetwork(request, asset) {
  let response
  try {
    response = await fetch(new Request(request, { cache: 'no-store', ...(asset ? { credentials: 'omit' } : {}) }))
  } catch (error) {
    if (request.signal.aborted) throw error
    const cached = await cacheRead(request, asset)
    if (cached) return cached
    throw error
  }
  // Optional storage must never hold a successful network response indefinitely.
  await bounded(cacheWrite(request, response, asset)).catch(() => {})
  return response
}
