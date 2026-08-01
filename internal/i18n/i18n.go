package i18n

import (
	"fmt"
	"strings"
)

type Translator struct {
	language string
}

type translations [3]string

type prefixTranslation struct {
	source string
	texts  translations
}

func New(language string) Translator {
	switch language {
	case "ru", "es", "zh":
		return Translator{language: language}
	default:
		return Translator{language: "en"}
	}
}

func (t Translator) Language() string {
	return t.language
}

func (t Translator) Text(source string) string {
	if t.language == "en" || source == "" {
		return source
	}
	if values, ok := messages[source]; ok {
		return values[t.index()]
	}
	if values, ok := extraMessages[source]; ok {
		return values[t.index()]
	}
	if values, ok := helpMessages[source]; ok {
		return values[t.index()]
	}
	for _, rule := range prefixMessages {
		if strings.HasPrefix(source, rule.source) {
			return rule.texts[t.index()] + strings.TrimPrefix(source, rule.source)
		}
	}
	return source
}

func (t Translator) Format(source string, values ...interface{}) string {
	return fmt.Sprintf(t.Text(source), values...)
}

func (t Translator) index() int {
	switch t.language {
	case "ru":
		return 0
	case "es":
		return 1
	default:
		return 2
	}
}

var messages = map[string]translations{
	"Status":                  {"Статус", "Estado", "状态"},
	"Groups":                  {"Группы", "Grupos", "组"},
	"Users":                   {"Пользователи", "Usuarios", "用户"},
	"Policies":                {"Политики", "Políticas", "策略"},
	"Engines":                 {"Хранилища", "Motores", "存储引擎"},
	"Auth Methods":            {"Методы входа", "Métodos de autenticación", "认证方法"},
	"Audit":                   {"Аудит", "Auditoría", "审计"},
	"Tokens":                  {"Токены", "Tokens", "令牌"},
	"Sections":                {"Разделы", "Secciones", "分区"},
	"Actions":                 {"Действия", "Acciones", "操作"},
	"Preview":                 {"Просмотр", "Vista previa", "预览"},
	"Editor":                  {"Редактор", "Editor", "编辑器"},
	"Confirm":                 {"Подтверждение", "Confirmar", "确认"},
	"Saving":                  {"Сохранение", "Guardando", "保存中"},
	"Help":                    {"Подсказка", "Ayuda", "帮助"},
	"Diff":                    {"Изменения", "Cambios", "更改"},
	"Choose an action":        {"Выберите действие", "Elija una acción", "选择操作"},
	"Save":                    {"Сохранить", "Guardar", "保存"},
	"Cancel":                  {"Отмена", "Cancelar", "取消"},
	"Discard":                 {"Отбросить", "Descartar", "放弃"},
	"Create":                  {"Создать", "Crear", "创建"},
	"Delete":                  {"Удалить", "Eliminar", "删除"},
	"Update":                  {"Обновить", "Actualizar", "更新"},
	"Enable":                  {"Включить", "Habilitar", "启用"},
	"Disable":                 {"Отключить", "Deshabilitar", "禁用"},
	"Revoke":                  {"Отозвать", "Revocar", "撤销"},
	"Restore":                 {"Восстановить", "Restaurar", "恢复"},
	"Tune":                    {"Настроить", "Ajustar", "调整"},
	"Submit key":              {"Отправить ключ", "Enviar clave", "提交密钥"},
	"yes":                     {"да", "sí", "是"},
	"no":                      {"нет", "no", "否"},
	"true":                    {"true", "true", "true"},
	"false":                   {"false", "false", "false"},
	"Health":                  {"Состояние", "Salud", "健康状态"},
	"Leader":                  {"Лидер", "Líder", "主节点"},
	"Seal status":             {"Статус seal", "Estado de sellado", "密封状态"},
	"Raft peers":              {"Узлы Raft", "Nodos Raft", "Raft 节点"},
	"Initialized":             {"Инициализирован", "Inicializado", "已初始化"},
	"Sealed":                  {"Запечатан", "Sellado", "已密封"},
	"Standby":                 {"Резервный", "En espera", "备用"},
	"Version":                 {"Версия", "Versión", "版本"},
	"Cluster":                 {"Кластер", "Clúster", "集群"},
	"Cluster ID":              {"ID кластера", "ID del clúster", "集群 ID"},
	"Enterprise":              {"Enterprise", "Enterprise", "企业版"},
	"Server time":             {"Время сервера", "Hora del servidor", "服务器时间"},
	"HA enabled":              {"HA включён", "HA habilitado", "HA 已启用"},
	"This node active":        {"Этот узел активен", "Este nodo está activo", "当前节点活动"},
	"Leader address":          {"Адрес лидера", "Dirección del líder", "主节点地址"},
	"Cluster address":         {"Адрес кластера", "Dirección del clúster", "集群地址"},
	"Active since":            {"Активен с", "Activo desde", "活动起始时间"},
	"Raft committed":          {"Raft committed", "Raft confirmado", "Raft 已提交"},
	"Raft applied":            {"Raft applied", "Raft aplicado", "Raft 已应用"},
	"Seal type":               {"Тип seal", "Tipo de sellado", "密封类型"},
	"Shares":                  {"Доли", "Partes", "密钥份额"},
	"Threshold":               {"Порог", "Umbral", "阈值"},
	"Storage":                 {"Хранилище", "Almacenamiento", "存储"},
	"Configuration":           {"Конфигурация", "Configuración", "配置"},
	"Name":                    {"Имя", "Nombre", "名称"},
	"Type":                    {"Тип", "Tipo", "类型"},
	"Members":                 {"Участники", "Miembros", "成员"},
	"Nested groups":           {"Вложенные группы", "Grupos anidados", "嵌套组"},
	"Parent groups":           {"Родительские группы", "Grupos padre", "父组"},
	"Metadata":                {"Метаданные", "Metadatos", "元数据"},
	"Created":                 {"Создан", "Creado", "已创建"},
	"Updated":                 {"Обновлён", "Actualizado", "已更新"},
	"Modify index":            {"Индекс изменения", "Índice de modificación", "修改索引"},
	"State":                   {"Состояние", "Estado", "状态"},
	"Disabled":                {"Отключён", "Deshabilitado", "已禁用"},
	"Direct policies":         {"Прямые политики", "Políticas directas", "直接策略"},
	"Direct groups":           {"Прямые группы", "Grupos directos", "直接组"},
	"Inherited groups":        {"Унаследованные группы", "Grupos heredados", "继承组"},
	"Aliases":                 {"Псевдонимы", "Alias", "别名"},
	"Entity ID":               {"ID entity", "ID de entity", "Entity ID"},
	"Username":                {"Имя пользователя", "Usuario", "用户名"},
	"Auth mount":              {"Точка аутентификации", "Montaje de autenticación", "认证挂载"},
	"Token policies":          {"Политики токена", "Políticas del token", "令牌策略"},
	"Token TTL":               {"TTL токена", "TTL del token", "令牌 TTL"},
	"Token max TTL":           {"Макс. TTL токена", "TTL máximo del token", "令牌最大 TTL"},
	"Token type":              {"Тип токена", "Tipo de token", "令牌类型"},
	"Policy":                  {"Политика", "Política", "策略"},
	"Path":                    {"Путь", "Ruta", "路径"},
	"Description":             {"Описание", "Descripción", "说明"},
	"Default TTL":             {"TTL по умолчанию", "TTL predeterminado", "默认 TTL"},
	"Max TTL":                 {"Максимальный TTL", "TTL máximo", "最大 TTL"},
	"Listing visibility":      {"Видимость в списке", "Visibilidad del listado", "列表可见性"},
	"Options":                 {"Параметры", "Opciones", "选项"},
	"Accessor":                {"Accessor", "Accessor", "Accessor"},
	"Local":                   {"Локальный", "Local", "本地"},
	"Seal wrap":               {"Seal wrap", "Seal wrap", "密封包装"},
	"Display name":            {"Отображаемое имя", "Nombre visible", "显示名称"},
	"Entity":                  {"Entity", "Entity", "Entity"},
	"TTL":                     {"TTL", "TTL", "TTL"},
	"Expires":                 {"Истекает", "Caduca", "过期时间"},
	"Orphan":                  {"Без родителя", "Huérfano", "孤立"},
	"Uses remaining":          {"Осталось использований", "Usos restantes", "剩余使用次数"},
	"Role":                    {"Роль", "Rol", "角色"},
	"Account":                 {"Учётная запись", "Cuenta", "账户"},
	"File":                    {"Файл", "Archivo", "文件"},
	"Forced":                  {"Принудительно", "Forzado", "强制"},
	"Bytes":                   {"Байт", "Bytes", "字节"},
	"Result":                  {"Результат", "Resultado", "结果"},
	"No data":                 {"Нет данных", "Sin datos", "无数据"},
	"Unseal key":              {"Ключ unseal", "Clave unseal", "Unseal 密钥"},
	"Snapshot file":           {"Файл snapshot", "Archivo de snapshot", "快照文件"},
	"Force restore":           {"Принудительное восстановление", "Forzar restauración", "强制恢复"},
	"New password":            {"Новый пароль", "Nueva contraseña", "新密码"},
	"Confirm password":        {"Подтверждение пароля", "Confirmar contraseña", "确认密码"},
	"Role JSON":               {"JSON роли", "JSON del rol", "角色 JSON"},
	"SSH mount":               {"SSH mount", "Montaje SSH", "SSH 挂载"},
	"Role name":               {"Имя роли", "Nombre del rol", "角色名称"},
	"Mount path":              {"Путь mount", "Ruta del montaje", "挂载路径"},
	"Engine type":             {"Тип хранилища", "Tipo de motor", "引擎类型"},
	"Auth type":               {"Тип аутентификации", "Tipo de autenticación", "认证类型"},
	"Device path":             {"Путь устройства", "Ruta del dispositivo", "设备路径"},
	"Device type":             {"Тип устройства", "Tipo de dispositivo", "设备类型"},
	"Seal cluster":            {"Запечатать кластер", "Sellar clúster", "密封集群"},
	"Unseal cluster":          {"Распечатать кластер", "Desellar clúster", "解除集群密封"},
	"Save snapshot":           {"Сохранить snapshot", "Guardar snapshot", "保存快照"},
	"Restore snapshot":        {"Восстановить snapshot", "Restaurar snapshot", "恢复快照"},
	"Edit group":              {"Изменить группу", "Editar grupo", "编辑组"},
	"Create group":            {"Создать группу", "Crear grupo", "创建组"},
	"Delete group":            {"Удалить группу", "Eliminar grupo", "删除组"},
	"Edit access":             {"Изменить доступ", "Editar acceso", "编辑访问权限"},
	"Change password":         {"Изменить пароль", "Cambiar contraseña", "修改密码"},
	"Create user":             {"Создать пользователя", "Crear usuario", "创建用户"},
	"Revoke access":           {"Отозвать доступ", "Revocar acceso", "撤销访问权限"},
	"Delete entity":           {"Удалить entity", "Eliminar entity", "删除 entity"},
	"Edit policy body":        {"Изменить текст политики", "Editar política", "编辑策略正文"},
	"Create policy":           {"Создать политику", "Crear política", "创建策略"},
	"Delete policy":           {"Удалить политику", "Eliminar política", "删除策略"},
	"Browse values":           {"Просмотреть значения", "Explorar valores", "浏览值"},
	"Enable engine":           {"Включить хранилище", "Habilitar motor", "启用引擎"},
	"Disable engine":          {"Отключить хранилище", "Deshabilitar motor", "禁用引擎"},
	"Edit role":               {"Изменить роль", "Editar rol", "编辑角色"},
	"Create role":             {"Создать роль", "Crear rol", "创建角色"},
	"Delete role":             {"Удалить роль", "Eliminar rol", "删除角色"},
	"Tune auth method":        {"Настроить метод входа", "Ajustar autenticación", "调整认证方法"},
	"Enable auth method":      {"Включить метод входа", "Habilitar autenticación", "启用认证方法"},
	"Disable auth method":     {"Отключить метод входа", "Deshabilitar autenticación", "禁用认证方法"},
	"Enable audit device":     {"Включить аудит", "Habilitar auditoría", "启用审计设备"},
	"Disable audit device":    {"Отключить аудит", "Deshabilitar auditoría", "禁用审计设备"},
	"Revoke token":            {"Отозвать токен", "Revocar token", "撤销令牌"},
	"Press r to retry":        {"Нажмите r для повтора", "Pulse r para reintentar", "按 r 重试"},
	"No items":                {"Нет элементов", "No hay elementos", "无项目"},
	"Draft updated":           {"Черновик обновлён", "Borrador actualizado", "草稿已更新"},
	"Review action":           {"Проверка действия", "Revisar acción", "检查操作"},
	"Editor closed":           {"Редактор закрыт", "Editor cerrado", "编辑器已关闭"},
	"Action canceled":         {"Действие отменено", "Acción cancelada", "操作已取消"},
	"Continue editing":        {"Продолжение редактирования", "Continuar editando", "继续编辑"},
	"Changes discarded":       {"Изменения отброшены", "Cambios descartados", "更改已放弃"},
	"Saved and verified":      {"Сохранено и проверено", "Guardado y verificado", "已保存并验证"},
	"Confirmation required":   {"Требуется подтверждение", "Se requiere confirmación", "需要确认"},
	"Read-only property":      {"Свойство только для чтения", "Propiedad de solo lectura", "只读属性"},
	"Editor will open here":   {"Здесь откроется редактор", "El editor se abrirá aquí", "编辑器将在此打开"},
	"[loading…]":              {"[загрузка…]", "[cargando…]", "[加载中…]"},
	"[no items]":              {"[нет элементов]", "[sin elementos]", "[无项目]"},
	"[not loaded]":            {"[не загружено]", "[no cargado]", "[未加载]"},
	"[nothing selected]":      {"[ничего не выбрано]", "[nada seleccionado]", "[未选择]"},
	"[opening editor…]":       {"[открытие редактора…]", "[abriendo editor…]", "[正在打开编辑器…]"},
	"[saving and verifying…]": {"[сохранение и проверка…]", "[guardando y verificando…]", "[正在保存并验证…]"},
}

var prefixMessages = []prefixTranslation{
	{"Invalid format: ", translations{"Некорректный формат: ", "Formato no válido: ", "格式无效："}},
	{"Loading ", translations{"Загрузка ", "Cargando ", "正在加载 "}},
	{"Opening ", translations{"Открытие ", "Abriendo ", "正在打开 "}},
	{"Editor unavailable: ", translations{"Редактор недоступен: ", "Editor no disponible: ", "编辑器不可用："}},
	{"Save failed: ", translations{"Ошибка сохранения: ", "Error al guardar: ", "保存失败："}},
	{"Edit group ", translations{"Изменить группу ", "Editar grupo ", "编辑组 "}},
	{"Edit access for ", translations{"Изменить доступ для ", "Editar acceso de ", "编辑访问权限："}},
	{"Change password for ", translations{"Изменить пароль для ", "Cambiar contraseña de ", "修改密码："}},
	{"Delete group ", translations{"Удалить группу ", "Eliminar grupo ", "删除组 "}},
	{"Revoke userpass access for ", translations{"Отозвать userpass-доступ для ", "Revocar acceso userpass de ", "撤销 userpass 访问："}},
	{"Delete identity entity ", translations{"Удалить identity entity ", "Eliminar identity entity ", "删除 identity entity "}},
	{"Edit ACL policy ", translations{"Изменить ACL-политику ", "Editar política ACL ", "编辑 ACL 策略 "}},
	{"Delete ACL policy ", translations{"Удалить ACL-политику ", "Eliminar política ACL ", "删除 ACL 策略 "}},
	{"Disable secrets engine ", translations{"Отключить secrets engine ", "Deshabilitar secrets engine ", "禁用 secrets engine "}},
	{"Edit SSH role ", translations{"Изменить SSH-роль ", "Editar rol SSH ", "编辑 SSH 角色 "}},
	{"Delete SSH role ", translations{"Удалить SSH-роль ", "Eliminar rol SSH ", "删除 SSH 角色 "}},
	{"Tune auth method ", translations{"Настроить метод входа ", "Ajustar autenticación ", "调整认证方法 "}},
	{"Disable auth method ", translations{"Отключить метод входа ", "Deshabilitar autenticación ", "禁用认证方法 "}},
	{"Disable audit device ", translations{"Отключить аудит ", "Deshabilitar auditoría ", "禁用审计设备 "}},
	{"Revoke token ", translations{"Отозвать токен ", "Revocar token ", "撤销令牌 "}},
}
