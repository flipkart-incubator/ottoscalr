import requests
import json
import datetime
import csv
from urllib.parse import quote

prometheus_url = "http://localhost:8181"  # Replace with your Prometheus server URL

# Set the step size (e.g., every 10 seconds)
step = '30s'

# Function to fetch data for a given workload and save it to a CSV file
def fetch_and_save_workload_data(namespace, workload, workload_type):
    # Adjust the time range as needed
    start_date = datetime.datetime(2023, 3, 27)
    end_date = datetime.datetime(2023, 4, 22)
    delta = datetime.timedelta(days=1)

    current_date = start_date
    while current_date <= end_date:
        start_time = current_date.strftime('%Y-%m-%dT%H:%M:%SZ')
        current_date += delta
        end_time = current_date.strftime('%Y-%m-%dT%H:%M:%SZ')

        # Build the query for the specific workload
        query = f"""sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate{{namespace="{namespace}"}} * on (namespace, pod) group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel{{namespace="{namespace}",workload="{workload}", workload_type="{workload_type}"}}) by(namespace, workload, workload_type)"""
        print(query)

        # URL-encode the query
        encoded_query = quote(query)

        # Create the query URL
        query_url = f'{prometheus_url}/api/v1/query_range?query={encoded_query}&start={start_time}&end={end_time}&step={step}'

        response = requests.get(query_url)
        data = json.loads(response.text)

        results = data['data']['result']

        # Save the results to a CSV file
        with open(f'{workload}.csv', 'a', newline='') as csvfile:
            fieldnames = ['namespace', 'workload', 'workload_type', 'timestamp', 'value']
            writer = csv.DictWriter(csvfile, fieldnames=fieldnames)

            for result in results:
                for value_pair in result['values']:
                    writer.writerow({
                        'namespace': namespace,
                        'workload': workload,
                        'workload_type': workload_type,
                        'timestamp': value_pair[0],
                        'value': value_pair[1]
                    })

# Get the list of workloads with current utilization greater than 50
query = """(sum(node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate * on (namespace, pod) group_left(workload, workload_type) namespace_workload_pod:kube_pod_owner:relabel{workload_type="deployment"}) by(namespace, workload, workload_type)) * on (namespace, workload,workload_type) group_left count(namespace_workload_pod:kube_pod_owner:relabel) by (namespace, workload, workload_type) > 50"""

# URL-encode the query
encoded_query = quote(query)

# Create the query URL
query_url = f'{prometheus_url}/api/v1/query?query={encoded_query}'

response = requests.get(query_url)
data = json.loads(response.text)

workloads = data['data']['result']

# Iterate over the workloads and fetch data for each workload
for workload in workloads:
    namespace = workload['metric']['namespace']
    workload_name = workload['metric']['workload']
    workload_type = workload['metric']['workload_type']
    fetch_and_save_workload_data(namespace, workload_name, workload_type)

print("Data written to individual workload CSV files.")
