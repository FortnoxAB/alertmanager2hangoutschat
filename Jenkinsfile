node('go') {
	def newTag = ''
	def tag = ''
	def gitTag = ''

	try {
		stage('Checkout'){
			dir('/go/src/github.com/fortnoxab/alertmanager2hangoutschat'){
				checkout scm
				step([$class: 'StashNotifier'])
				gitTag = sh(script: 'git tag -l --contains HEAD', returnStdout: true).trim()
			}
		}



		dir('/go/src/github.com/fortnoxab/alertmanager2hangoutschat'){

			stage('Fetch dependencies'){
				sh('dep ensure')
			}
			stage('Run test'){
				sh('go test -v ./...')
			}

			if(gitTag != ''){
				tag = gitTag
			}

			if( tag != ''){
				strippedTag = tag.replaceFirst('v', '')
				stage('Build the application'){
					echo "Building with docker tag ${strippedTag}"
					sh('CGO_ENABLED=0 GOOS=linux go build')
				}

				stage('Generate docker image'){
					image = docker.build('fortnox/alertmanager2hangoutschat:'+strippedTag, '--pull .')
				}

				stage('Push docker image'){
					docker.withRegistry("https://quay.io", 'docker-registry') {
						image.push()
					}
				}
			}
		}

		currentBuild.result = 'SUCCESS' 
	} catch(err) {
		currentBuild.result = 'FAILED'      // Set result of currentBuild !Important!
		step([$class: 'StashNotifier'])
		throw err
	}

	step([$class: 'StashNotifier'])
}

