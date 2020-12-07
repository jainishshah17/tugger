@Library('jenkins.shared.library') _

environment {
  HELM_IMAGE = "infoblox/helm:3.2.4-5b243a2"
  REGISTRY = "core-harbor-prod.sdp.infoblox.com"
  TAG = sh(script: "git describe --always --long --tags", returnStdout: true).trim()
}

pipeline {
  agent {
    label 'ubuntu_docker_label'
  }
  stages {
    stage("Build Image") {
      sh "docker build . -t tugger:${version}"
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
      dir("chart") {
        sh '''
          sed -i "s!repository: .*!repository: $REGISTRY/infoblox/tugger!g" tugger/values.yaml
          sed -i "s!tag: .*!tag: $TAG!g" tugger/values.yaml
        '''
        withAWS(credentials: "CICD_HELM", region: "us-east-1") {
          sh '''
            docker run --rm \
                -v $(pwd):/pkg \
                $HELM_IMAGE package /pkg/tugger --version $TAG -d /pkg
            docker run --rm \
                -e AWS_REGION \
                -e AWS_ACCESS_KEY_ID \
                -e AWS_SECRET_ACCESS_KEY \
                -v $WORKSPACE/$DIRECTORY/helm:/pkg \
                $HELM_IMAGE s3 push /pkg/tugger infobloxcto
            cd ..
            mkdir build
            echo "repo=infobloxcto" > $WORKSPACE/${DIRECTORY}/build/build.properties
            echo "chart=tugger" >> $WORKSPACE/${DIRECTORY}/build/build.properties
            echo "messageFormat=s3-artifact" >> $WORKSPACE/${DIRECTORY}/build/build.properties
            echo "customFormat=true" >> $WORKSPACE/${DIRECTORY}/build/build.properties
          '''
        }
      }
    }
    stage("Push Chart") {
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
      archiveArtifacts artifacts: 'build.properties'    }
      archiveArtifacts artifacts: 'chart/*.tgz'
  }
}
