const reviewedInterfaceTranslations = {
  ar: core('إعدادات الحساب', 'الإجراءات', 'النشاط', 'المظهر', 'الأرشيف', 'إنشاء رسالة', 'جهات الاتصال', 'حذف', 'حفظ', 'بحث', 'إلغاء', 'اكتب رسالتك…', 'إلى:'),
  cs: core('Nastavení účtu', 'Akce', 'Aktivita', 'Vzhled', 'Archiv', 'Napsat', 'Kontakty', 'Smazat', 'Uložit', 'Hledat', 'Zrušit', 'Napište zprávu…', 'Komu:'),
  de: core('Kontoeinstellungen', 'Aktionen', 'Aktivität', 'Erscheinungsbild', 'Archiv', 'Verfassen', 'Kontakte', 'Löschen', 'Speichern', 'Suchen', 'Abbrechen', 'Nachricht schreiben…', 'An:'),
  es: core('Configuración de la cuenta', 'Acciones', 'Actividad', 'Apariencia', 'Archivo', 'Redactar', 'Contactos', 'Eliminar', 'Guardar', 'Buscar', 'Cancelar', 'Escribe tu mensaje…', 'Para:'),
  'es-419': core('Configuración de la cuenta', 'Acciones', 'Actividad', 'Apariencia', 'Archivo', 'Redactar', 'Contactos', 'Eliminar', 'Guardar', 'Buscar', 'Cancelar', 'Escribe tu mensaje…', 'Para:'),
  fr: core('Paramètres du compte', 'Actions', 'Activité', 'Apparence', 'Archives', 'Écrire', 'Contacts', 'Supprimer', 'Enregistrer', 'Rechercher', 'Annuler', 'Écrivez votre message…', 'À :'),
  gl: core('Configuración da conta', 'Accións', 'Actividade', 'Aparencia', 'Arquivo', 'Redactar', 'Contactos', 'Eliminar', 'Gardar', 'Buscar', 'Cancelar', 'Escribe a túa mensaxe…', 'Para:'),
  he: core('הגדרות חשבון', 'פעולות', 'פעילות', 'מראה', 'ארכיון', 'כתיבת הודעה', 'אנשי קשר', 'מחיקה', 'שמירה', 'חיפוש', 'ביטול', 'כתבו את ההודעה שלכם…', 'אל:'),
  hu: core('Fiókbeállítások', 'Műveletek', 'Tevékenység', 'Megjelenés', 'Archívum', 'Levélírás', 'Névjegyek', 'Törlés', 'Mentés', 'Keresés', 'Mégse', 'Írja meg üzenetét…', 'Címzett:'),
  ja: core('アカウント設定', '操作', 'アクティビティ', '外観', 'アーカイブ', '作成', '連絡先', '削除', '保存', '検索', 'キャンセル', 'メッセージを入力…', '宛先:'),
  ko: core('계정 설정', '작업', '활동', '모양', '보관', '작성', '연락처', '삭제', '저장', '검색', '취소', '메시지를 입력하세요…', '받는 사람:'),
  pl: core('Ustawienia konta', 'Akcje', 'Aktywność', 'Wygląd', 'Archiwum', 'Napisz', 'Kontakty', 'Usuń', 'Zapisz', 'Szukaj', 'Anuluj', 'Napisz wiadomość…', 'Do:'),
  pt: core('Definições da conta', 'Ações', 'Atividade', 'Aspeto', 'Arquivo', 'Escrever', 'Contactos', 'Eliminar', 'Guardar', 'Pesquisar', 'Cancelar', 'Escreva a sua mensagem…', 'Para:'),
  'pt-BR': core('Configurações da conta', 'Ações', 'Atividade', 'Aparência', 'Arquivo', 'Escrever', 'Contatos', 'Excluir', 'Salvar', 'Pesquisar', 'Cancelar', 'Escreva sua mensagem…', 'Para:'),
  ru: core('Настройки учётной записи', 'Действия', 'Активность', 'Внешний вид', 'Архив', 'Написать', 'Контакты', 'Удалить', 'Сохранить', 'Поиск', 'Отмена', 'Напишите сообщение…', 'Кому:'),
  tr: core('Hesap ayarları', 'Eylemler', 'Etkinlik', 'Görünüm', 'Arşiv', 'Oluştur', 'Kişiler', 'Sil', 'Kaydet', 'Ara', 'İptal', 'İletinizi yazın…', 'Kime:'),
  uk: core('Налаштування облікового запису', 'Дії', 'Активність', 'Вигляд', 'Архів', 'Написати', 'Контакти', 'Видалити', 'Зберегти', 'Пошук', 'Скасувати', 'Напишіть повідомлення…', 'Кому:'),
  'zh-CN': core('账户设置', '操作', '活动', '外观', '归档', '写邮件', '联系人', '删除', '保存', '搜索', '取消', '输入邮件内容…', '收件人：'),
  'zh-TW': core('帳戶設定', '操作', '活動', '外觀', '封存', '撰寫', '聯絡人', '刪除', '儲存', '搜尋', '取消', '輸入訊息…', '收件者：'),
}

function core(accountSettings, actions, activity, appearance, archive, compose, contacts, remove, save, search, cancel, message, recipient) {
  return Object.freeze({
    'Account settings': accountSettings,
    Actions: actions,
    Activity: activity,
    Appearance: appearance,
    Archive: archive,
    Compose: compose,
    Contacts: contacts,
    Delete: remove,
    Save: save,
    Search: search,
    Cancel: cancel,
    'Write your message…': message,
    'To:': recipient,
  })
}

export default Object.freeze(reviewedInterfaceTranslations)
