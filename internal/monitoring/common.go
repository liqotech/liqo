package monitoring

func createInitMap() map[string]bool {
	retMap := make(map[string]bool)

	for i := LiqoComponent(0); i < lastComponent; i++ {
		for j := EventType(0); j < lastEvent; j++ {
			retMap[i.String()+j.String()] = true
		}
	}

	return retMap
}

func createConsistencyEventMap(initValue bool) map[string]bool {
	retMap := make(map[string]bool)

	for i := LiqoComponent(0); i < lastComponent; i++ {
		for j := EventType(0); j < lastEvent; j++ {
			retMap[i.String()+j.String()] = initValue
		}
	}

	return retMap
}
