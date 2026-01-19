package protocol

const (
	CmdTemplatesList    = "templates.list"
	CmdTemplatesInspect = "templates.inspect"
)

type TemplatesInspectRequest struct {
	Name string `json:"name"`
}
