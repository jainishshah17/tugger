@Library('jenkins.shared.library') _

pipeline {
  agent {
    label 'ubuntu_docker_label'
  }
  environment {
    HELM_IMAGE = "infoblox/helm:3.2.4-5b243a2"
    REGISTRY = "core-harbor-prod.sdp.infoblox.com"
    VERSION = sh(script: "git describe --always --long --tags", returnStdout: true).trim()
    TAG = "${env.VERSION}-j${env.BUILD_NUMBER}"
  }
  stages {
    stage("Build Image") {
      steps {
        sh 'docker build . -t tugger:$TAG'
      }
    }
    stage("Push Image") {
      when {
        anyOf {
          branch 'master'
          branch 'jenkinsfile'
        }
      }
      steps {
        script {
          signDockerImage('tugger', env.TAG, 'infoblox')
        }
      }
    }
    stage("Package Chart") {
      steps {
        dir("chart") {
          sh '''
            sed -i "s!repository: .*!repository: $REGISTRY/infoblox/tugger!g" tugger/values.yaml
          '''
          withAWS(credentials: "CICD_HELM", region: "us-east-1") {
            sh '''
              docker run --rm \
                  -e AWS_REGION \
                  -e AWS_ACCESS_KEY_ID \
                  -e AWS_SECRET_ACCESS_KEY \
                  -v $(pwd):/pkg \
                  $HELM_IMAGE package /pkg/tugger --app-version $TAG --version $TAG -d /pkg
            '''
          }
        }
      }
    }
    stage("Push Chart") {
      steps {
        withAWS(credentials: "CICD_HELM", region: "us-east-1") {
          sh '''
            chart_file=tugger-$TAG.tgz
            docker run --rm \
                -e AWS_REGION \
                -e AWS_ACCESS_KEY_ID \
                -e AWS_SECRET_ACCESS_KEY \
                -v $(pwd)/chart:/pkg \
                $HELM_IMAGE s3 push /pkg/$chart_file infobloxcto
            echo "repo=infobloxcto" > build.properties
            echo "chart=$chart_file" >> build.properties
            echo "messageFormat=s3-artifact" >> build.properties
            echo "customFormat=true" >> build.properties
          '''
        }
        archiveArtifacts artifacts: 'build.properties'
        archiveArtifacts artifacts: 'chart/*.tgz'
      }
    }
  }
  post {
    success {
       finalizeBuild()
    }
  }
}
