package txpool

type Service struct {
}

func NewService() *Service {
	srv := &Service{}

	return srv
}

func (s *Service) Start() {

}

func (s *Service) Stop() error {
	return nil
}

func (s *Service) Status() error {
	return nil
}

func (s *Service) SendRawTx() error {
	return nil
}
